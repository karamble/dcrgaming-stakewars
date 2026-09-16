# Comic weapon artwork

`comic-weapons-v1.png` is the original generated atlas, created with the built-in
image generation tool. It contains eleven weapon/tool illustrations in gameplay
order: bazooka, grenade, cluster bomb, shotgun, Uzi, fire punch, dynamite, mine,
teleport, rope launcher, jetpack.

The original PNG is embedded by `sprites.go`. Individually authored bounds
extract eleven sprites at startup, preserving transparency without modifying
the source artwork. Both desktop and offline renderers use these images in
the clickable weapon tray and hover previews. GPU sprite textures are cached
separately from the mutable terrain texture.

Generation prompt is preserved in `prompt-v1.txt`.

`comic-projectiles-v1.png` supplies rocket, shotgun-pellet, Uzi-round and
cluster-bomblet sprites through `projectiles.go`. It was generated with the
built-in image tool using `projectile-prompt-v1.txt`. Rocket art rotates with
velocity; grenades/bomblets tumble. Instant-hit shot effects use the actual
simulation ray endpoints and remain presentation-only.

## Expansion atlas

`comic-expansion-v1.png` contains the twelve additional inventory icons in
simulation order, starting with mortar. `comic-expansion-projectiles-v1.png`
contains eight matching flight/charge sprites: mortar shell, bouncing bomb,
drill rocket, homing missile, aerial bomb, flame, remote charge and heavy
cluster. The bison uses its character sprite while running; the bat is shown
for its strike and the parachute canopy is drawn above a deployed Stakey.

Both were generated with the built-in imagegen tool. Exact prompts are saved
in `expansion-prompt-v1.txt` and `expansion-projectiles-prompt-v1.txt`.
`expansion.go` embeds the originals and extracts sprite regions, trimming only
transparent margins at runtime. No source PNG is rewritten.

## Mining drill

`comic-mining-drill-v1.png` is the original built-in imagegen generation.
`comic-mining-drill-v2.png` is the built-in imagegen transparency edit used by
`mining_drill.go`, preserving generated alpha. Original generation prompt:
`drill-prompt-v1.txt`; edit prompt: `drill-prompt-v2.txt`. Both are original
StakeWars comic assets; the final sprite is used in the arsenal and held tool.
