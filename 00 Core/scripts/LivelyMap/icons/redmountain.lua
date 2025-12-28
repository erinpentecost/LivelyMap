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
local aux_util   = require('openmw_aux.util')
local imageAtlas = require('scripts.LivelyMap.h3.imageAtlas')
local iutil      = require("scripts.LivelyMap.icons.iutil")

local color      = util.color.rgb(200 / 255, 60 / 255, 20 / 255)

-- TODO: only enable this if the main quest exists
-- TODO: change the color to not-red once the main quest finishes


local smokeAtlas = imageAtlas.constructAtlas({
    totalTiles = 30,
    tilesPerRow = 6,
    atlasPath = "textures/LivelyMap/smoke_atlas.png",
    tileSize = util.vector2(256, 256), -- 1279 length, 1536 width. 5 rows, 6 cols
    create = true,
})
smokeAtlas:spawn({
    anchor = util.vector2(0.5, 0.5),
    color = color,
})

local mapUp = false

interfaces.LivelyMapDraw.onMapMoved(function(_)
    mapUp = true
end)

interfaces.LivelyMapDraw.onMapHidden(function(_)
    mapUp = false
end)

local baseSize = util.vector2(512, 512)
local redMountainPos = util.vector3(20254, 69038, 2000) -- TODO: fix Z, it's wrong
local animIndex = 1
local animSpeed = 1 / 30

local smokeIcon = {
    element = smokeAtlas:getElement(),
    pos = function()
        return redMountainPos
    end,
    ---@param posData ViewportData
    onDraw = function(_, posData)
        smokeAtlas:getElement().layout.props.visible = true
        smokeAtlas:getElement().layout.props.position = posData.viewportPos.pos

        smokeAtlas:getElement().layout.props.size = baseSize * iutil.distanceScale(posData)
        smokeAtlas:setTile(animIndex)
        smokeAtlas:getElement():update()
    end,
    onHide = function()
        smokeAtlas:getElement().layout.props.visible = false
        smokeAtlas:getElement():update()
    end,
}

local delay = -5
local function onUpdate(dt)
    -- Don't run if the map is not up.
    if not mapUp then
        -- Make sure we always run on the first frame
        -- of the map going up.
        delay = -5
        return
    end

    -- Fake a duration if we're paused.
    if dt <= 0 then
        dt = 1 / 60
    end

    -- Only run about every second.
    delay = delay - dt
    if delay > 0 then
        return
    end
    delay = animSpeed
    animIndex = ((animIndex + 1) % 30) + 1
    smokeAtlas:setTile(animIndex)
    smokeAtlas:getElement():update()
end


interfaces.LivelyMapDraw.registerIcon(smokeIcon)

return {
    engineHandlers = {
        onUpdate = onUpdate,
    },
}
