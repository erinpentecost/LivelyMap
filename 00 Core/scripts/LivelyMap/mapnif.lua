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
local MOD_NAME = require("scripts.LivelyMap.ns")
local aux_util = require('openmw_aux.util')
local util = require('openmw.util')
local pself = require("openmw.self")

-- This script is attached to the 3d floating map objects.
-- It should be responsible for adding annotations and cleaning
-- them up.

-- mapData holds read-only map metadata for this extent.
-- It also contains "player", which is the player that
-- summoned it.
local mapData = nil

local function onStart(initData)
    if initData ~= nil then
        mapData = initData
    end
end
local function onSave()
    return mapData
end

local function onActive()
    print("activated " .. aux_util.deepToString(mapData, 3))
end

local function onInactive()
    -- this is called after the map is destroyed,
    -- so it's probably too late to clean up annotations at this point.
    print("inactivated " .. aux_util.deepToString(mapData, 3))
end

local function getBounds()
    -- this is called on the map object
    local verts = pself:getBoundingBox().vertices

    local minX, maxX = verts[1].x, verts[1].x
    local minY, maxY = verts[1].y, verts[1].y
    local minZ, maxZ = verts[1].z, verts[1].z

    for i = 2, #verts do
        local v = verts[i]
        minX = math.min(minX, v.x)
        maxX = math.max(maxX, v.x)
        minY = math.min(minY, v.y)
        maxY = math.max(maxY, v.y)
        minZ = math.min(minZ, v.z)
        maxZ = math.max(maxZ, v.z)
    end

    return {
        bottomLeft  = util.vector3(minX, minY, minZ),
        bottomRight = util.vector3(maxX, minY, minZ),
        topLeft     = util.vector3(minX, maxY, minZ),
        topRight    = util.vector3(maxX, maxY, minZ),
    }
end

local CELL_SIZE = 128 * 64 -- 8192
local function worldToMapMeshTransform(bounds, extents)
    -- Compute width/height in world space.
    -- these are the dimensions of the map object.
    local worldWidth  = bounds.bottomRight.x - bounds.bottomLeft.x
    local worldHeight = bounds.topLeft.y - bounds.bottomLeft.y
    local scaleZ      = 1
    if worldHeight < 0 or worldWidth < 0 then
        error("negative dimensions: height:" .. tostring(worldHeight) .. ", width:" .. tostring(worldWidth))
    end

    -- Compute width/height in cells
    -- extents are inclusive.
    local cellWidth  = extents.Right - extents.Left + 1
    local cellHeight = extents.Top - extents.Bottom + 1

    -- Compute scale
    local scaleX     = worldWidth / (cellWidth * CELL_SIZE)
    local scaleY     = worldHeight / (cellHeight * CELL_SIZE)

    -- Compute translation to align bottom-left of extents to bottom-left of bounds
    local moveX      = bounds.bottomLeft.x - extents.Left * CELL_SIZE * scaleX
    local moveY      = bounds.bottomLeft.y - extents.Bottom * CELL_SIZE * scaleY
    local moveZ      = bounds.bottomLeft.z

    -- Return real util.transform
    return util.transform.identity
        * util.transform.scale(scaleX, scaleY, scaleZ)
        * util.transform.move(util.vector3(moveX, moveY, moveZ))
end





local function worldToRelativeMapTransform2(bounds)
    if mapData == nil or mapData.Extents == nil then
        error("missing map extents")
    end

    local e          = mapData.Extents

    local mapWidth   = e.Right - e.Left
    local mapHeight  = e.Top - e.Bottom

    local meshWidth  = bounds.bottomRight.x - bounds.bottomLeft.x
    local meshHeight = bounds.topLeft.y - bounds.bottomLeft.y

    return
    -- relative map → mesh quad
        util.transform.move(bounds.bottomLeft)
        *
        util.transform.scale(meshWidth, meshHeight, 1)
        *
        -- cell → relative map
        util.transform.scale(
            1 / mapWidth,
            1 / mapHeight,
            1
        )
        *
        util.transform.move(
            -e.Left,
            -e.Bottom,
            0
        )
        *
        -- world → cell
        util.transform.scale(
            1 / CELL_SIZE,
            1 / CELL_SIZE,
            1
        )
end

-- onMapMoved is called when the map is placed or moved.
-- this should move/create all the annotations it owns
local function onMapMoved(data)
    if data == nil then
        error("onTeleported data is nil")
    end
    if data.position == nil then
        error("onTeleported data.position is nil")
    end
    mapData = data

    mapData.bounds = getBounds()
    mapData.worldToMapMeshTransform = worldToMapMeshTransform(mapData.bounds, mapData.Extents)

    print("onTeleported")

    mapData.player:sendEvent(MOD_NAME .. "onMapMoved", mapData)
end

return {
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
    },
    engineHandlers = {
        onActive = onActive,
        onInactive = onInactive,
        onInit = onStart,
        onSave = onSave,
        onLoad = onStart,
    }
}
