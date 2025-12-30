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
local util       = require('openmw.util')
local pself      = require("openmw.self")
local mutil      = require("scripts.LivelyMap.mutil")
local nearby     = require('openmw.nearby')
local settings   = require("scripts.LivelyMap.settings")
local async      = require("openmw.async")
local camera     = require("openmw.camera")
local myui       = require('scripts.LivelyMap.pcp.myui')
local interfaces = require('openmw.interfaces')
local ui         = require('openmw.ui')
local async      = require("openmw.async")

local function distanceScale(posData)
    local dist = (camera.getPosition() - posData.mapWorldPos):length()
    dist = util.clamp(dist, 100, 500)
    local max = 1
    if posData.hovering then
        max = 1.25
    end
    return util.remap(dist, 100, 500, max, 0.5)
end

local function hoverTextLayout(text, color)
    return {
        template = interfaces.MWUI.templates.textHeader,
        type = ui.TYPE.Text,
        alignment = ui.ALIGNMENT.End,
        props = {
            textAlignV = ui.ALIGNMENT.Center,
            relativePosition = util.vector2(0, 0.5),
            text = text,
            textSize = 20,
            textColor = color or myui.interactiveTextColors.normal.default,
        }
    }
end

return {
    distanceScale = distanceScale,
    hoverTextLayout = hoverTextLayout,
}
