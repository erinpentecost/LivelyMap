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
local interfaces   = require('openmw.interfaces')
local ui           = require('openmw.ui')
local util         = require('openmw.util')
local pself        = require("openmw.self")
local aux_util     = require('openmw_aux.util')
local imageAtlas   = require('scripts.LivelyMap.h3.imageAtlas')
local iutil        = require("scripts.LivelyMap.icons.iutil")

-- "/home/ern/workspace/LivelyMap/cmd/h3/make_atlas.sh" -i  "/home/ern/workspace/LivelyMap/00 Core/textures/LivelyMap/arrow.png" -o "/home/ern/workspace/LivelyMap/00 Core/textures/LivelyMap/arrow_atlas.dds" -r 20 -c 18
local compassAtlas = imageAtlas.constructAtlas({
    totalTiles = 360,
    tilesPerRow = 18,
    atlasPath = "textures/LivelyMap/arrow_atlas.dds",
    tileSize = util.vector2(100, 100),
    create = true,
})
compassAtlas:spawn({
    layer = "HUD",
    anchor = util.vector2(0.5, 0.5),
})

local function adjustedYaw(deg)
    local yaw = math.deg(deg)

    if yaw < 0 then yaw = util.remap(yaw, -180, 0, 181, 360) end

    return util.clamp(util.round(yaw), 1, 360)
end

local compassIcon = {
    pos = function()
        return pself.position
    end,
    facing = function()
        return pself.rotation:apply(util.vector3(0.0, 1.0, 0.0)):normalize()
    end,
    onDraw = function(posData)
        compassAtlas:getElement().layout.props.visible = true
        compassAtlas:getElement().layout.props.position = posData.viewportPos

        if not posData.viewportFacing then
            compassAtlas:getElement().layout.props.visible = false
            compassAtlas:getElement():update()
            return
        end

        compassAtlas:getElement().layout.props.size = util.vector2(50, 50) * iutil.distanceScale(posData.mapWorldPos)

        local angle = math.atan2(posData.viewportFacing.x, -1 * posData.viewportFacing.y)

        -- Convert to degrees, where 0° = East, 90° = North.
        local deg = adjustedYaw(angle)
        --print(deg .. " - " .. tostring(posData.viewportFacing))

        compassAtlas:setTile(deg)
        compassAtlas:getElement():update()
    end,
    onHide = function()
        compassAtlas:getElement().layout.props.visible = false
        compassAtlas:getElement():update()
    end,
}

interfaces.LivelyMapDraw.registerIcon(compassIcon)
