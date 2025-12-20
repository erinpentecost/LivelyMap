--[[
LivelyMap for OpenMW.
Copyright (C) Erin Pentecost 2025

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
local iutil      = require("scripts.LivelyMap.icons.iutil")


local debugEnabled = false
settings.subscribe(async:callback(function(_, key)
    if key == "psoUnlock" then
        debugEnabled = settings.psoUnlock
    end
end))
debugEnabled = settings.psoUnlock



local debugIcons = {}

local function makeDebugPips()
    for x = -2, 2, 1 do
        for y = -2, 2, 1 do
            if not (x == 0 and y == 0) then
                local offset = util.vector3(
                    x * mutil.CELL_SIZE / 2,
                    y * mutil.CELL_SIZE / 2,
                    100 * mutil.CELL_SIZE
                )

                local pip = ui.create {
                    name = "debug_" .. tostring(x) .. "_" .. tostring(y),
                    type = ui.TYPE.Image,
                    layer = iutil.layer,
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
                local worldPos = function()
                    if not debugEnabled then
                        return nil
                    end
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
                table.insert(debugIcons, {
                    pip = pip,
                    pos = worldPos,
                    onDraw = function(posData)
                        if not debugEnabled then
                            pip.layout.props.visible = false
                            return
                        end
                        pip.layout.props.size = util.vector2(32, 32) * iutil.distanceScale(posData)
                        pip.layout.props.visible = true
                        pip.layout.props.position = posData.viewportPos
                        pip:update()
                    end,
                    onHide = function()
                        pip.layout.props.visible = false
                        pip:update()
                    end,
                })
            end
        end
    end
end

makeDebugPips()
for _, icon in ipairs(debugIcons) do
    interfaces.LivelyMapDraw.registerIcon(icon)
end
