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
local util     = require('openmw.util')
local pself    = require("openmw.self")
local mutil    = require("scripts.LivelyMap.mutil")
local nearby   = require('openmw.nearby')
local settings = require("scripts.LivelyMap.settings")
local async    = require("openmw.async")
local camera   = require("openmw.camera")


local function distanceScale(mapWorldPos)
    local dist = (camera.getPosition() - mapWorldPos):length()
    dist = util.clamp(dist, 100, 500)
    return util.remap(dist, 100, 500, 1, 0.5)
end

return {
    distanceScale = distanceScale,
}
