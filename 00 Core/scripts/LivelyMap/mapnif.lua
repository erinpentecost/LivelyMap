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
local MOD_NAME = require("scripts.LivelyMap.ns")
local mutil = require("scripts.LivelyMap.mutil")
local aux_util = require('openmw_aux.util')
local util = require('openmw.util')
local pself = require("openmw.self")

-- This script is attached to the 3d floating map objects.

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

-- Returns a util.transform that maps raw world-space positions to map mesh positions
local function worldToMapMeshTransform(bounds, extents)
    -- Width and height of map bounds in world units
    local worldWidth  = bounds.bottomRight.x - bounds.bottomLeft.x
    local worldHeight = bounds.topLeft.y - bounds.bottomLeft.y
    local scaleZ      = 1

    -- Width and height in cells (inclusive)
    local cellWidth   = extents.Right - extents.Left + 1
    local cellHeight  = extents.Top - extents.Bottom + 1

    -- Scale factors: world → map mesh
    local scaleX      = worldWidth / (cellWidth * mutil.CELL_SIZE)
    local scaleY      = worldHeight / (cellHeight * mutil.CELL_SIZE)

    -- Translation: align extents bottom-left with bounds bottom-left
    local moveX       = bounds.bottomLeft.x - extents.Left * mutil.CELL_SIZE * scaleX
    local moveY       = bounds.bottomLeft.y - extents.Bottom * mutil.CELL_SIZE * scaleY
    local moveZ       = bounds.bottomLeft.z

    -- Compose single transform
    return util.transform.identity
        * util.transform.scale(scaleX, scaleY, scaleZ)
        * util.transform.move(util.vector3(moveX, moveY, moveZ))
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
        onInit = onStart,
        onSave = onSave,
        onLoad = function(initData)
            onStart(initData)
            if initData ~= nil then
                initData.player:sendEvent(MOD_NAME .. "onMapMoved", initData)
            end
        end,
    }
}
