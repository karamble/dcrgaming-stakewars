#!/usr/bin/env bash
# Full two-player StakeWars financial-authority acceptance test.
#
# This script uses fixed, public simnet-only wallet seeds. It creates isolated
# state below /tmp, starts two independent wallets, bridges and game processes,
# and proves both cooperative payout and unilateral mature recovery.
set -Eeuo pipefail

SW_REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SRC_ROOT=${SRC_ROOT:-$(cd "$SW_REPO/../.." && pwd)}
SDK_REPO=${SDK_REPO:-$SRC_ROOT/karamble/dcrgaming-sdk}
PULSE_REPO=${PULSE_REPO:-$SRC_ROOT/karamble/dcrpulse}
DCRD_REPO=${DCRD_REPO:-$SRC_ROOT/decred/dcrd}
WALLET_REPO=${WALLET_REPO:-$SRC_ROOT/decred/dcrwallet}
BR_REPO=${BR_REPO:-$SRC_ROOT/companyzero/bisonrelay}
BRCLIENT_REPO=${BRCLIENT_REPO:-$SRC_ROOT/karamble/brclientd}
TEST_ROOT=${STAKEWARS_SIMNET_ROOT:-/tmp/stakewars-simnet-acceptance}
BIN=$TEST_ROOT/bin
RUN=$TEST_ROOT/run
LOG=$RUN/logs

RPC_USER=stakewars-simnet
RPC_PASS=stakewars-simnet-pass
WALLET_PASS=123
DASHBOARD_PASS=stakewars-simnet-dashboard-password
PULSE_IMAGE=${PULSE_IMAGE:-alpine:3.22}
PULSE1=sw-stakewars-simnet-pulse1-$$
PULSE2=sw-stakewars-simnet-pulse2-$$
PIDS=()

say() { printf '\n==> %s\n' "$*"; }
die() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  docker rm -f "$PULSE1" "$PULSE2" >/dev/null 2>&1 || true
  if ((${#PIDS[@]})); then
    kill "${PIDS[@]}" >/dev/null 2>&1 || true
    wait "${PIDS[@]}" >/dev/null 2>&1 || true
  fi
  if ((status != 0)); then
    printf '\nLogs retained at %s\n' "$LOG" >&2
    docker logs "$PULSE1" 2>&1 | tail -80 >&2 || true
    docker logs "$PULSE2" 2>&1 | tail -80 >&2 || true
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM
trap 'printf "FAIL: line %d: %s\n" "$LINENO" "$BASH_COMMAND" >&2' ERR

for cmd in go curl jq docker; do command -v "$cmd" >/dev/null || die "$cmd is required"; done
for dir in "$SDK_REPO" "$PULSE_REPO/dashboard" "$DCRD_REPO" "$WALLET_REPO" "$BR_REPO" "$BRCLIENT_REPO"; do
  test -d "$dir" || die "missing source tree: $dir"
done
docker image inspect "$PULSE_IMAGE" >/dev/null 2>&1 || die "Docker image $PULSE_IMAGE is required"

if [[ ${SKIP_BUILD:-0} == 1 ]]; then
  rm -rf "$RUN"
  for binary in dcrd dcrwallet brserver brclientd dcrpulse stakewars-simnet-player; do
    test -x "$BIN/$binary" || die "SKIP_BUILD=1 but $BIN/$binary is missing"
  done
else
  rm -rf "$TEST_ROOT"
fi
mkdir -p "$BIN" "$LOG" "$RUN"/{dcrd,brserver,wallet1,wallet2,brclient1,brclient2,player1,player2}

run_bg() {
  local name=$1; shift
  "$@" >"$LOG/$name.log" 2>&1 &
  PIDS+=("$!")
}

wait_http() {
  local url=$1
  for _ in $(seq 1 240); do
    curl -fsS "$url" >/dev/null 2>&1 && return 0
    sleep .25
  done
  die "timed out waiting for $url"
}

wait_file() {
  local path=$1
  for _ in $(seq 1 240); do test -s "$path" && return 0; sleep .25; done
  die "timed out waiting for $path"
}

dcrd_rpc() {
  local method=$1 params=${2:-'[]'}
  curl -fsS --cacert "$RUN/dcrd/rpc.cert" --user "$RPC_USER:$RPC_PASS" \
    -H content-type:application/json \
    --data "$(jq -nc --arg method "$method" --argjson params "$params" '{jsonrpc:"1.0",id:"stakewars",method:$method,params:$params}')" \
    https://127.0.0.1:19556/ | jq -e '.error == null' >/dev/null
}

dcrd_result() {
  local method=$1 params=${2:-'[]'}
  curl -fsS --cacert "$RUN/dcrd/rpc.cert" --user "$RPC_USER:$RPC_PASS" \
    -H content-type:application/json \
    --data "$(jq -nc --arg method "$method" --argjson params "$params" '{jsonrpc:"1.0",id:"stakewars",method:$method,params:$params}')" \
    https://127.0.0.1:19556/ | jq -e '.result'
}

mine() { dcrd_result generate "[$1]" >/dev/null; }

wallet_result() {
  local number=$1 port=$2 method=$3 params=${4:-'[]'}
  curl -fsS --cacert "$RUN/wallet$number/rpc.cert" --user "$RPC_USER:$RPC_PASS" \
    -H content-type:application/json \
    --data "$(jq -nc --arg method "$method" --argjson params "$params" '{jsonrpc:"1.0",id:"stakewars",method:$method,params:$params}')" \
    "https://127.0.0.1:$port/" | jq -e '.result'
}

br_api() {
  local number=$1 port=$2 path=$3; shift 3
  curl -fsS --cacert "$RUN/brclient$number/data/simnet/rpc/rpc.cert" \
    --cert "$RUN/brclient$number/data/simnet/rpc/rpc-client.cert" \
    --key "$RUN/brclient$number/data/simnet/rpc/rpc-client.key" \
    "$@" "https://127.0.0.1:$port$path"
}

pulse_get() {
  local number=$1 port=$2 path=$3
  curl -fsS -b "$RUN/pulse$number/cookie.jar" "http://127.0.0.1:$port/api$path"
}

pulse_post() {
  local number=$1 port=$2 path=$3 data=$4
  curl -fsS -b "$RUN/pulse$number/cookie.jar" \
    -H "Origin: http://127.0.0.1:$port" -H content-type:application/json \
    --data "$data" "http://127.0.0.1:$port/api$path"
}

wait_pulse_jq() {
  local number=$1 port=$2 path=$3 filter=$4
  local body
  for _ in $(seq 1 360); do
    body=$(pulse_get "$number" "$port" "$path" 2>/dev/null || true)
    if test -n "$body" && jq -e "$filter" >/dev/null 2>&1 <<<"$body"; then printf '%s\n' "$body"; return 0; fi
    sleep .5
  done
  die "timed out waiting for pulse $number $path: $filter"
}

wait_player_jq() {
  local port=$1 filter=$2 body
  for _ in $(seq 1 360); do
    body=$(curl -fsS "http://127.0.0.1:$port/status" 2>/dev/null || true)
    if test -n "$body" && jq -e "$filter" >/dev/null 2>&1 <<<"$body"; then printf '%s\n' "$body"; return 0; fi
    sleep .5
  done
  die "timed out waiting for player $port: $filter"
}

say "Build current sources"
if [[ ${SKIP_BUILD:-0} != 1 ]]; then
  (cd "$DCRD_REPO" && go build -o "$BIN/dcrd" .)
  (cd "$WALLET_REPO" && go build -o "$BIN/dcrwallet" .)
  (cd "$BR_REPO" && go build -o "$BIN/brserver" ./brserver)
  (cd "$BRCLIENT_REPO" && go build -o "$BIN/brclientd" ./cmd/brclientd)
  (cd "$PULSE_REPO/dashboard" && CGO_ENABLED=0 go build -o "$BIN/dcrpulse" ./cmd/dcrpulse)
  (cd "$SW_REPO" && go build -o "$BIN/stakewars-simnet-player" ./cmd/stakewars-simnet-player)
fi

cat >"$RUN/dcrd.conf" <<EOF
[Application Options]
simnet=1
appdata=$RUN/dcrd
datadir=$RUN/dcrd/data
logdir=$RUN/dcrd/logs
rpcuser=$RPC_USER
rpcpass=$RPC_PASS
rpclisten=127.0.0.1:19556
listen=127.0.0.1:19555
rpccert=$RUN/dcrd/rpc.cert
rpckey=$RUN/dcrd/rpc.key
miningaddr=SsXciQNTo3HuV5tX3yy4hXndRWgLMRVC7Ah
debuglevel=warn
EOF
cat >"$RUN/brserver.conf" <<EOF
root=$RUN/brserver
routedmessages=$RUN/brserver/routedmessages
paidrvs=$RUN/brserver/paidrvs
listen=127.0.0.1:19443
[payment]
scheme=free
pushrateatoms=0.001
pushratebytes=1
[log]
logfile=$RUN/brserver/brserver.log
debuglevel=warn
profiler=
EOF

run_bg dcrd "$BIN/dcrd" -C "$RUN/dcrd.conf"
run_bg brserver "$BIN/brserver" -cfg "$RUN/brserver.conf"
wait_file "$RUN/dcrd/rpc.cert"
for _ in $(seq 1 120); do dcrd_rpc getblockcount && break; sleep .25; done
mine 2

for number in 1 2; do
  port=$((19547 + number * 10))
  grpc=$((19548 + number * 10))
  cat >"$RUN/wallet$number.conf" <<EOF
[Application Options]
simnet=1
appdata=$RUN/wallet$number
logdir=$RUN/wallet$number/logs
pass=$WALLET_PASS
rpcconnect=127.0.0.1:19556
cafile=$RUN/dcrd/rpc.cert
rpclisten=127.0.0.1:$port
grpclisten=127.0.0.1:$grpc
rpccert=$RUN/wallet$number/rpc.cert
rpckey=$RUN/wallet$number/rpc.key
clientcafile=$RUN/wallet$number/rpc.cert
username=$RPC_USER
password=$RPC_PASS
debuglevel=warn
EOF
done

# Public simnet fixture seeds. Never use these wallets on another network.
printf 'y\nn\ny\nb280922d2cffda44648346412c5ec97f429938105003730414f10b01e1402eac\n\n\n' | "$BIN/dcrwallet" -C "$RUN/wallet1.conf" --create >/dev/null
printf 'y\nn\ny\n4242424242424242424242424242424242424242424242424242424242424242\n\n\n' | "$BIN/dcrwallet" -C "$RUN/wallet2.conf" --create >/dev/null
run_bg wallet1 "$BIN/dcrwallet" -C "$RUN/wallet1.conf"
run_bg wallet2 "$BIN/dcrwallet" -C "$RUN/wallet2.conf"
for _ in $(seq 1 240); do wallet_result 1 19557 getbalance >/dev/null 2>&1 && wallet_result 2 19567 getbalance >/dev/null 2>&1 && break; sleep .25; done
mine 32
for _ in $(seq 1 240); do
  balance=$(wallet_result 1 19557 getbalance 2>/dev/null | jq -r '.totalspendable // (.balances[0].spendable // 0)' || true)
  awk "BEGIN {exit !($balance > 20)}" && break
  sleep .25
done
wallet2_address=$(wallet_result 2 19567 getnewaddress | jq -r .)
wallet_result 1 19557 sendtoaddress "$(jq -nc --arg address "$wallet2_address" '[$address,20]')" >/dev/null
mine 2

say "Start two independent Bison Relay identities"
run_bg brclient1 "$BIN/brclientd" --appdata="$RUN/brclient1" --simnet --brserver=127.0.0.1:19443 --brserverdirect --payscheme=free --clientrpc.listen=127.0.0.1:19760 --clientrpc.issueclientcert --status.listen=127.0.0.1:19761 --mcp.mcplisten=127.0.0.1:19762
run_bg brclient2 "$BIN/brclientd" --appdata="$RUN/brclient2" --simnet --brserver=127.0.0.1:19443 --brserverdirect --payscheme=free --clientrpc.listen=127.0.0.1:19770 --clientrpc.issueclientcert --status.listen=127.0.0.1:19771 --mcp.mcplisten=127.0.0.1:19772
wait_file "$RUN/brclient1/data/simnet/rpc/rpc-client.cert"
wait_file "$RUN/brclient2/data/simnet/rpc/rpc-client.cert"
br_api 1 19760 /create-identity -H content-type:application/json --data '{"nick":"stakewars-one","name":"StakeWars One"}' >/dev/null
br_api 2 19770 /create-identity -H content-type:application/json --data '{"nick":"stakewars-two","name":"StakeWars Two"}' >/dev/null
for _ in $(seq 1 240); do br_api 1 19761 /public-identity >/dev/null 2>&1 && br_api 2 19771 /public-identity >/dev/null 2>&1 && break; sleep .25; done

invite=$(br_api 1 19761 /invites/create -H content-type:application/json --data '{}' | jq -r .inviteBytes)
br_api 2 19771 /invites/accept -H content-type:application/json --data "$(jq -nc --arg inviteBytes "$invite" '{inviteBytes:$inviteBytes}')" >/dev/null
for _ in $(seq 1 240); do
  one=$(br_api 1 19761 /contacts 2>/dev/null || true)
  two=$(br_api 2 19771 /contacts 2>/dev/null || true)
  jq -e '.. | strings | select(. == "stakewars-two")' >/dev/null 2>&1 <<<"$one" && \
    jq -e '.. | strings | select(. == "stakewars-one")' >/dev/null 2>&1 <<<"$two" && break
  sleep .25
done
identity=$(br_api 2 19771 /public-identity | jq -r .identity)
if [[ ! $identity =~ ^[0-9a-fA-F]{64}$ ]]; then identity=$(printf %s "$identity" | base64 -d | od -An -tx1 | tr -d ' \n'); fi
gcid=$(br_api 1 19761 /gc/create -H content-type:application/json --data '{"name":"StakeWars simnet acceptance"}' | jq -r .id)
br_api 1 19761 "/gc/$gcid/invite" -H content-type:application/json --data "$(jq -nc --arg uid "$identity" '{uid:$uid}')" >/dev/null
invite_id=
for _ in $(seq 1 240); do
  invite_id=$(br_api 2 19771 /gc/invites 2>/dev/null | jq -r --arg group "$gcid" '.invites[]? | select(.gcid == $group) | .id' | head -1 || true)
  test -n "$invite_id" && break
  sleep .25
done
test -n "$invite_id" || die "group invitation did not arrive"
br_api 2 19771 /gc/invites/accept -H content-type:application/json --data "$(jq -nc --argjson iid "$invite_id" '{iid:$iid}')" >/dev/null

say "Start two independent dcrpulse bridges"
for number in 1 2; do
  mkdir -p "$RUN/pulse$number/app-data/control" "$RUN/pulse$number/dashboard-data/wallets/simnet/default-wallet"
  printf '{"name":"default-wallet","appdata":"/app-data/dcrwallet","network":"simnet","epoch":1}\n' >"$RUN/pulse$number/app-data/control/selected.json"
done

start_pulse() {
  local number=$1 wallet_rpc=$2 wallet_grpc=$3 br_rpc=$4 br_status=$5 http=$6 gaming=$7 container=$8
  docker run -d --rm --name "$container" --network host --user 1000:1000 \
    -v "$BIN/dcrpulse:/usr/local/bin/dcrpulse:ro" \
    -v "$RUN/dcrd/rpc.cert:/certs/dcrd.cert:ro" \
    -v "$RUN/wallet$number/rpc.cert:/certs/wallet.cert:ro" \
    -v "$RUN/pulse$number/app-data:/app-data" \
    -v "$RUN/pulse$number/dashboard-data:/dashboard-data" \
    -v "$RUN/brclient$number:/brclient-data:ro" \
    -e DCRD_RPC_HOST=127.0.0.1 -e DCRD_RPC_PORT=19556 -e DCRD_RPC_USER="$RPC_USER" -e DCRD_RPC_PASS="$RPC_PASS" -e DCRD_RPC_CERT=/certs/dcrd.cert \
    -e DCRWALLET_RPC_HOST=127.0.0.1 -e DCRWALLET_RPC_PORT="$wallet_rpc" -e DCRWALLET_GRPC_PORT="$wallet_grpc" -e DCRWALLET_RPC_USER="$RPC_USER" -e DCRWALLET_RPC_PASS="$RPC_PASS" -e DCRWALLET_RPC_CERT=/certs/wallet.cert \
    -e BRCLIENTD_HOST=127.0.0.1 -e BRCLIENTD_PORT="$br_rpc" -e BRCLIENTD_STATUS_PORT="$br_status" -e BRCLIENTD_DATA_DIR=/brclient-data \
    -e PORT="$http" -e GAMING_BRIDGE_HOST=127.0.0.1 -e GAMING_BRIDGE_PORT="$gaming" \
    -e DCRPULSE_LOG_LEVEL=warn -e DASHBOARD_HOST_BIND=127.0.0.1 -e MCP_ENABLE=false \
    "$PULSE_IMAGE" /usr/local/bin/dcrpulse >/dev/null
}
start_pulse 1 19557 19558 19760 19761 19680 19690 "$PULSE1"
start_pulse 2 19567 19568 19770 19771 19681 19691 "$PULSE2"
wait_http http://127.0.0.1:19680/api/auth/status
wait_http http://127.0.0.1:19681/api/auth/status

configure_pulse() {
  local number=$1 http=$2 gaming=$3 cookie="$RUN/pulse$number/cookie.jar"
  curl -fsS -c "$cookie" -H "Origin: http://127.0.0.1:$http" -H content-type:application/json \
    --data "$(jq -nc --arg password "$DASHBOARD_PASS" '{password:$password}')" "http://127.0.0.1:$http/api/auth/setup" >/dev/null
  pulse_post "$number" "$http" /br/gaming/settings '{"enabled":true,"registeredGames":["stakewars"],"policies":{"stakewars":{"name":"StakeWars","account":"default","perTableCapDcr":1,"perDayCapDcr":10,"approvalTimeoutSecs":600}}}' >/dev/null
  credential=$(pulse_post "$number" "$http" /br/gaming/credential '{"game":"stakewars"}')
  jq -n --arg network simnet --arg host 127.0.0.1 --arg port "$gaming" \
    --arg cert "$(jq -r .certPem <<<"$credential")" --arg key "$(jq -r .keyPem <<<"$credential")" \
    --arg bridge "$(jq -r .bridgeCertPem <<<"$credential")" \
    '{network:$network,host:$host,port:$port,client_certificate:$cert,client_private_key:$key,bridge_certificate:$bridge}' >"$RUN/player$number/bridge.json"
  chmod 600 "$RUN/player$number/bridge.json" "$cookie"
}
configure_pulse 1 19680 19690
configure_pulse 2 19681 19691

run_bg player1 "$BIN/stakewars-simnet-player" -appdata "$RUN/player1" -listen 127.0.0.1:19801
run_bg player2 "$BIN/stakewars-simnet-player" -appdata "$RUN/player2" -listen 127.0.0.1:19802
wait_player_jq 19801 '.connected == true' >/dev/null
wait_player_jq 19802 '.connected == true' >/dev/null

approve_pending() {
  local number=$1 port=$2 table=$3 kind=$4
  body=$(wait_pulse_jq "$number" "$port" /br/gaming/spends ".pending | any(.tableId == \"$table\" and .depositKind == \"$kind\")")
  id=$(jq -r --arg table "$table" --arg kind "$kind" '.pending[] | select(.tableId == $table and .depositKind == $kind) | .id' <<<"$body" | head -1)
  pulse_post "$number" "$port" /br/gaming/spends/decide "$(jq -nc --arg id "$id" --arg pass "$WALLET_PASS" '{id:$id,approve:true,passphrase:$pass}')" >/dev/null
}

table_request() {
  local blocks=$1
  jq -nc --arg gcid "$gcid" --argjson blocks "$blocks" '{game:"stakewars",gcid:$gcid,buyinAtoms:100000,seats:2,openBlocks:$blocks,funds:{refundBlocks:288,admissionAtoms:1000000,admissionBlocks:2016,tableBondAtoms:0,tableBondBlocks:0}}'
}

say "Create an unfilled table for unilateral recovery"
recovery_table=$(pulse_post 1 19680 /br/gaming/table "$(table_request 4)")
recovery_sid=$(jq -r .sid <<<"$recovery_table")
recovery_until=$(jq -r .until <<<"$recovery_table")
approve_pending 1 19680 "$recovery_sid" seatbond
mine 1
height=$(dcrd_result getblockcount | jq -r .)
if ((height <= recovery_until)); then mine $((recovery_until - height + 1)); fi
recovery_row=$(wait_pulse_jq 1 19680 /br/gaming/recovery ".deposits | any(.table == \"$recovery_sid\" and .kind == \"seatbond\")")
recovery_id=$(jq -r --arg table "$recovery_sid" '.deposits[] | select(.table == $table and .kind == "seatbond") | .id' <<<"$recovery_row" | head -1)

say "Create and fund a complete two-player table"
cooperative_table=$(pulse_post 1 19680 /br/gaming/table "$(table_request 20)")
cooperative_sid=$(jq -r .sid <<<"$cooperative_table")
cooperative_until=$(jq -r .until <<<"$cooperative_table")
cooperative_invite=$(jq -r .invite <<<"$cooperative_table")
pulse_post 2 19681 /br/gaming/invite "$(jq -nc --arg invite "$cooperative_invite" --arg gcid "$gcid" '{game:"stakewars",invite:$invite,gcid:$gcid}')" >/dev/null
approve_pending 1 19680 "$cooperative_sid" seatbond
approve_pending 2 19681 "$cooperative_sid" seatbond
mine 1
wait_player_jq 19801 ".match == \"$cooperative_sid\"" >/dev/null
wait_player_jq 19802 ".match == \"$cooperative_sid\"" >/dev/null
sleep 2
height=$(dcrd_result getblockcount | jq -r .)
if ((height <= cooperative_until)); then mine $((cooperative_until - height + 1)); fi
wait_player_jq 19801 ".match == \"$cooperative_sid\" and .canFund == true" >/dev/null
wait_player_jq 19802 ".match == \"$cooperative_sid\" and .canFund == true" >/dev/null
curl -fsS -X POST http://127.0.0.1:19801/fund >/dev/null
curl -fsS -X POST http://127.0.0.1:19802/fund >/dev/null
approve_pending 1 19680 "$cooperative_sid" stake
approve_pending 2 19681 "$cooperative_sid" stake
mine 1

for _ in $(seq 1 360); do
  s1=$(curl -fsS http://127.0.0.1:19801/status 2>/dev/null || true)
  s2=$(curl -fsS http://127.0.0.1:19802/status 2>/dev/null || true)
  h1=$(jq -r '.headHash // ""' <<<"$s1" 2>/dev/null || true)
  h2=$(jq -r '.headHash // ""' <<<"$s2" 2>/dev/null || true)
  if test -n "$h1" && test "$h1" = "$h2" && jq -e '.worldAgreed == true' >/dev/null <<<"$s1" && jq -e '.worldAgreed == true' >/dev/null <<<"$s2"; then break; fi
  sleep .5
done
test -n "${h1:-}" && test "$h1" = "${h2:-}" || die "players did not agree on the initial world"
initial_turn=$(jq -r .turn <<<"$s1")
if test "$(jq -r .activeSeat <<<"$s1")" = "$(jq -r .mine <<<"$s1")"; then active_port=19801; else active_port=19802; fi
curl -fsS -X POST -H content-type:application/json --data '{"mode":"fire"}' "http://127.0.0.1:$active_port/advance" >/dev/null
for _ in $(seq 1 360); do
  s1=$(curl -fsS http://127.0.0.1:19801/status 2>/dev/null || true); s2=$(curl -fsS http://127.0.0.1:19802/status 2>/dev/null || true)
  h1=$(jq -r '.headHash // ""' <<<"$s1" 2>/dev/null || true); h2=$(jq -r '.headHash // ""' <<<"$s2" 2>/dev/null || true)
  turn=$(jq -r '.turn // 0' <<<"$s1" 2>/dev/null || echo 0)
  test "$turn" -gt "$initial_turn" && test -n "$h1" && test "$h1" = "$h2" && break
  sleep .5
done
test "$h1" = "$h2" || die "players diverged after the fired turn"
if test "$(jq -r .activeSeat <<<"$s1")" = "$(jq -r .mine <<<"$s1")"; then active_port=19801; else active_port=19802; fi
curl -fsS -X POST -H content-type:application/json --data '{"mode":"surrender"}' "http://127.0.0.1:$active_port/advance" >/dev/null
wait_player_jq 19801 '.winner >= 0' >/dev/null
wait_player_jq 19802 '.winner >= 0' >/dev/null

say "Approve the exact cooperative payout on both bridges"
p1=$(wait_pulse_jq 1 19680 /br/gaming/payouts ".payouts | any(.table == \"$cooperative_sid\")")
p2=$(wait_pulse_jq 2 19681 /br/gaming/payouts ".payouts | any(.table == \"$cooperative_sid\")")
payout1=$(jq -r --arg table "$cooperative_sid" '.payouts[] | select(.table == $table) | .id' <<<"$p1" | head -1)
payout2=$(jq -r --arg table "$cooperative_sid" '.payouts[] | select(.table == $table) | .id' <<<"$p2" | head -1)
test "$payout1" = "$payout2" || die "bridges derived different payout IDs"
pulse_post 1 19680 /br/gaming/payouts "$(jq -nc --arg id "$payout1" --arg pass "$WALLET_PASS" '{id:$id,action:"approve",passphrase:$pass}')" >/dev/null
pulse_post 2 19681 /br/gaming/payouts "$(jq -nc --arg id "$payout2" --arg pass "$WALLET_PASS" '{id:$id,action:"approve",passphrase:$pass}')" >/dev/null
for _ in $(seq 1 360); do
  mempool=$(dcrd_result getrawmempool 2>/dev/null || echo '[]')
  test "$(jq length <<<"$mempool")" -gt 0 && break
  sleep .5
done
test "$(jq length <<<"$mempool")" -gt 0 || die "cooperative payout was not broadcast"
mine 1
wait_pulse_jq 1 19680 /br/gaming/payouts ".payouts | any(.id == \"$payout1\" and .state == \"confirmed\")" >/dev/null
wait_pulse_jq 2 19681 /br/gaming/payouts ".payouts | any(.id == \"$payout2\" and .state == \"confirmed\")" >/dev/null

say "Mature and recover the abandoned admission bond"
pulse_post 1 19680 /br/gaming/recovery "$(jq -nc --arg id "$recovery_id" '{id:$id,action:"close"}')" >/dev/null
row=$(pulse_get 1 19680 /br/gaming/recovery)
remaining=$(jq -r --arg id "$recovery_id" '.deposits[] | select(.id == $id) | .remainingBlocks' <<<"$row")
if ((remaining > 0)); then mine $((remaining + 1)); fi
row=$(wait_pulse_jq 1 19680 /br/gaming/recovery ".deposits | any(.id == \"$recovery_id\" and .canRecover == true)")
quote=$(pulse_post 1 19680 /br/gaming/recovery "$(jq -nc --arg id "$recovery_id" '{id:$id,action:"quote"}')")
quote_id=$(jq -r .id <<<"$quote")
result=$(pulse_post 1 19680 /br/gaming/recovery "$(jq -nc --arg id "$recovery_id" --arg quote "$quote_id" --arg pass "$WALLET_PASS" '{id:$id,quote:$quote,passphrase:$pass,action:"confirm"}')")
jq -e '.pending == true and (.txid | length == 64)' >/dev/null <<<"$result" || die "recovery was not broadcast"
mine 1
wait_pulse_jq 1 19680 /br/gaming/recovery ".deposits | any(.id == \"$recovery_id\" and .state == \"spent\")" >/dev/null

say "PASS: two wallets, two bridges, identical replay, cooperative payout, mature unilateral recovery"
