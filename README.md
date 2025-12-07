# LivelyMap

This is an OpenMW mod and tool.

## Install the mod

There are *Data Folders* in this folder that you need to add to your `openmw.cfg`.
These all have numbers as prefixes. At a minimum, you need to add `00 Core`.

You also need to add `00 Core/LivelyMap.omwaddon` as a plugin to `openmw.cfg`.
Don't add `LivelyMap.omwscripts` if you see it.

## Run the Sync Tool

1. Install Go: https://go.dev/doc/install
1. Run `sync.sh <location of my openmw.cfg>` or `sync.bat <location of my openmw.cfg>`.

This will generate all the required textures and metadata from your install.
It will also extract path data from your saved games.

## Customizing the map

You can specify a custom ramp.bmp by placing it in this folder and then running the sync tool. This should be a 1x512 resolution file, with the midpoint representing the water level.

## Updating the mod

Make sure `cmd/lively/lively` or `cmd/lively/lively.exe` are deleted after you pull in the new files.

## Showing the Map (debugging)

While in-game, bring up your console and type `lua map`. Optionally provide a number from 0 to 7, inclusive (ex: `lua map 6`).
