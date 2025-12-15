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

local compass    = ui.create {
    name = "compass",
    type = ui.TYPE.Image,
    layer = "HUD",
    icon = {
        worldPos = function()
            return pself.position
        end,
        defaultSize = util.vector2(32, 32),
    },
    props = {
        visible = false,
        position = util.vector2(100, 100),
        anchor = util.vector2(0.5, 0.5),
        size = util.vector2(32, 32),
        resource = ui.texture {
            path = "textures/compass.dds"
        }
    }
}

local function pos()
    return pself.position
end

return { compass, pos }
