# StakeWars cover v1

`stakewars-cover-v1.png` is the original generated comic cover, produced with the
built-in image-generation tool. The exact prompt is in `prompt-v1.txt`.
The image is embedded and drawn at startup before local simulation/scenery
initialization. A code-rendered loading indicator reflects initialization;
after preparation, Enter, Space or a click continues. There is no forced delay.
`-skip-cover` skips it for development. `-cover` includes it in screenshots;
settings and table-demo launches otherwise bypass it.

`stakewars-logo-v1.png` is the standalone transparent wordmark derived from the
cover using the built-in imagegen editing tool. Exact prompt:
`prompt-logo-v1.txt`. It is used by lobby/table/victory branding; the battlefield
HUD intentionally contains no logo. The original generated PNG is preserved.
