--[[
LivelyMap for OpenMW.
Copyright (C) 2025

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
]]
local interfaces = require('openmw.interfaces')
local ui         = require('openmw.ui')
local util       = require('openmw.util')
local pself      = require("openmw.self")
local mutil      = require("scripts.LivelyMap.mutil")
local nearby     = require('openmw.nearby')
local settings   = require("scripts.LivelyMap.settings")
local async      = require("openmw.async")
local iutil      = require("scripts.LivelyMap.iutil")

local debugPips  = {}

local function makeDebugPips()
    for x = -2, 2, 1 do
        for y = -2, 2, 1 do
            if not (x == 0 and y == 0) then
                local offset = util.vector3(
                    x * mutil.CELL_SIZE / 2,
                    y * mutil.CELL_SIZE / 2,
                    100 * mutil.CELL_SIZE
                )
                local worldPos = function()
                    local origin = pself.position + offset
                    local castResult = nearby.castRay(origin,
                        util.vector3(origin.x, origin.y, -100 * mutil.CELL_SIZE), {
                            collisionType = nearby.COLLISION_TYPE.HeightMap + nearby.COLLISION_TYPE.Water
                        })
                    if not castResult.hit then
                        return nil
                    end
                    return castResult.hitPos
                end
                local pip = ui.create {
                    name = "debug_" .. tostring(x) .. "_" .. tostring(y),
                    type = ui.TYPE.Image,
                    layer = "HUD",
                    props = {
                        visible = false,
                        position = util.vector2(100, 100),
                        anchor = util.vector2(0.5, 0.5),
                        size = util.vector2(32, 32),
                        resource = ui.texture {
                            path = "textures/detect_key_icon.dds"
                        }
                    }
                }
                local callbacks = {
                    onDraw = function(pos)
                        pip.layout.props.size = util.vector2(32, 32) * iutil.distanceScale(pos.mapWorldPos)
                    end
                }
                table.insert(debugPips, {
                    pip,
                    worldPos,
                    callbacks
                })
            end
        end
    end
end

local function updatePips(enabled)
    if enabled and interfaces.LivelyMapDraw then
        print("Debug pips enabled.")
        makeDebugPips()
        for _, icon in ipairs(debugPips) do
            interfaces.LivelyMapDraw.registerIcon(unpack(icon))
        end
    else
        print("Debug pips disabled.")
        for _, icon in ipairs(debugPips) do
            icon[1]:destroy()
            icon[2] = nil
        end
        debugPips = {}
    end
end

settings.subscribe(async:callback(function(_, key)
    if key == "psoUnlock" then
        updatePips(settings.psoUnlock)
    end
end))

updatePips(settings.psoUnlock)

-- have to return something so it's not garbage collected
return {}
