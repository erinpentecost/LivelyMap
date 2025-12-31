# LivelyMap

This is an OpenMW mod and tool.

## Installing the mod

### Data Folders and Plugin

There are *Data Folders* in this folder that you need to add to your `openmw.cfg`.
These all have numbers as prefixes. At a minimum, you need to add `00 Core`. Don't add more than one folder with the same number, they are exclusive.

You also need to add `00 Core/LivelyMap.omwaddon` as a plugin to `openmw.cfg`.
Don't add `LivelyMap.omwscripts` if you see it.

### Run the Sync Tool

1. Install Go: https://go.dev/doc/install
1. Run `sync.sh <location of my openmw.cfg>` or `sync.bat <location of my openmw.cfg>`.

This will generate all the required textures and metadata from your install.
It will also extract path data from your saved games.

You can specify a custom ramp.bmp by placing it in the same folder as this README and then running the sync tool. This should be a 1x512 resolution file, with the midpoint representing the water level.

## Updating the mod

Make sure `cmd/lively/lively` or `cmd/lively/lively.exe` are deleted after you pull in the new files.

## Using the map

Configure a key to bring the map up. You do this in the in-game mod settings. Then press that key to bring the map up. Press it again to take the map down (or press Escape or try to bring up your inventory or journal).

To *pan the map*: click, or click and drag, or use your arrow keys, or use your D-pad.

There are buttons in a bar at the top of the map. These do extra stuff.

### Parallax Shader Calibration

If you are using a parallax shader, the map will appear 3-dimensional! This is cool, but you will need to calibrate the vertical offset or icons will appear to float as you pan around. In the mod settings, enable *Parallax Calibration Mode*, then bring up the map. This adds three buttons to the top bar. The *-* reduces the offset width, and *+* increases it. The *down-arrow-into-a-bucket* toggles between only allowing icons to be pushed down into the mesh or if the offset width should be equally distributed between the top and bottom of the mesh. This is determined by how your shader is handling parallax offsets. When you're done, turn calibration mode off in your settings.

## Debugging

### Show the Map

While in-game, bring up your console and type `lua map`. Optionally provide a number from 0 to 7, inclusive (ex: `lua map 6`).

### Make a Marker

While in-game, bring up your console and type `lua marker`. Optionally provide an id (ex: `lua marker mymarker`). Markers will appear in the map.
