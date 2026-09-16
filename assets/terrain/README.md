# Generated comic terrain materials

Three source PNGs were generated with the built-in image tool:

- `slate-v1.png`: blue slate with turquoise mineral seams.
- `basalt-v1.png`: volcanic stone with ember cracks.
- `moss-v1.png`: green sedimentary rock and mineral pockets.

The exact prompts are saved beside them. `materials.go` embeds the originals
and samples mirrored, world-anchored tiles. The match seed selects the material.
The renderer clips these colors to the collision mask and adds exposed rims;
artwork never defines physics. No downloaded game screenshot or Hedgewars asset
was copied into these textures.
