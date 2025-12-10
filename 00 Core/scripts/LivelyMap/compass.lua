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

local ui = require('openmw.ui')
local util = require('openmw.util')

local compass = ui.create {
    name = "compass",
    type = ui.TYPE.Image,
    layer = "HUD",
    props = {
        relativePosition = util.vector2(0.5, 0.5),
        anchor = util.vector2(0, 0),
        resource = ui.texture {
            path = "compass.dds"
        }
    }
}
