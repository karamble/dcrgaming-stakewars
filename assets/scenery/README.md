# StakeWars scenery v1

Generated using the built-in image-generation tool. Exact prompts are saved in
`prompt-*-v1.txt`. Original generated PNGs are copied here without alpha edits.

- `sky-v1.png`: deep space, aurora and moon (opaque).
- `far-v1.png`: distant crystal mountains (transparent).
- `far-volcanic-v1.png`: volcanic mountain alternative (transparent).
- `mid-v1.png`: observatories and ruins (transparent).
- `mid-grove-v1.png`: bioluminescent grove alternative (transparent).
- `front-v1.png`: sparse foreground crystal/reed silhouettes (transparent).

Renderer speeds: sky 0.04×, far 0.18×, middle 0.43×, terrain 1×,
foreground 1.3×. Backgrounds also respond to vertical camera movement. Alternate
tiles are mirrored at draw time so identical edges meet. Theme selection uses
`Config.Seed % 3`, matching water/lava/slime; placement offsets use seed bits.
These assets never enter terrain masks or gameplay randomness. They are embedded
in the game binary, not loaded from the image generator's output directory.

`weather-v1.png` is an additional 4×2 transparent sprite atlas (exact prompt in
`prompt-weather-v1.txt`): two rain streaks, two leaves, ember, ash, mist, alien
leaf. Cells are sampled at runtime, preserving the source PNG. Weather drift
reads the simulation wind but changes presentation state only. Near and far
particles follow different camera/scroll speeds. No random gameplay calls are
used by the effects.
