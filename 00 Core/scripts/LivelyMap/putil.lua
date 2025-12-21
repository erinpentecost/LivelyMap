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
local MOD_NAME   = require("scripts.LivelyMap.ns")
local storage    = require('openmw.storage')
local util       = require('openmw.util')
local mutil      = require("scripts.LivelyMap.mutil")
local core       = require("openmw.core")
local pself      = require("openmw.self")
local aux_util   = require('openmw_aux.util')
local camera     = require("openmw.camera")
local ui         = require("openmw.ui")
local settings   = require("scripts.LivelyMap.settings")
local async      = require("openmw.async")
local interfaces = require('openmw.interfaces')

---This is a world-space position, but x and y are divided by CELL_LENGTH.
---@alias CellPos util.vector3

---X and Y are between 0 and 1, and are the relative locations on the mesh.
---@alias RelativeMeshPos util.vector2

---World space coordinate.
---@alias WorldSpacePos util.vector3

--- cellPosToRelativeMeshPos return mapPos, but shifted by the current map Extents
--- so the bottom left becomes 0,0 and top right becomes 1,1.
--- @param currentMapData MeshAnnotatedMapData
--- @param cellPos CellPos
--- @return RelativeMeshPos?
local function cellPosToRelativeMeshPos(currentMapData, cellPos)
    if currentMapData == nil then
        error("missing mapObject")
        return
    end
    if cellPos == nil then
        error("mapPos is nil")
    end
    if currentMapData.Extents == nil then
        error("mapPos.Extents is nil")
    end
    if cellPos.x < currentMapData.Extents.Left or cellPos.x > currentMapData.Extents.Right then
        error("offsetMapPos: x position is outside extents")
    end
    if cellPos.y < currentMapData.Extents.Bottom or cellPos.y > currentMapData.Extents.Top then
        error("offsetMapPos: x position is outside extents")
    end
    local x = util.remap(cellPos.x, currentMapData.Extents.Left, currentMapData.Extents.Right + 1, 0.0, 1.0)
    local y = util.remap(cellPos.y, currentMapData.Extents.Bottom, currentMapData.Extents.Top + 1, 0.0, 1.0)
    return util.vector2(x, y)
end


-- TODO: might be trash
-- relativeMeshPosToCellPos converts a relative [0,1) map position
-- back into an absolute cell position.
local function relativeMeshPosToCellPos(currentMapData, relMeshPos)
    if currentMapData == nil then
        error("missing mapObject")
    end
    if relMeshPos == nil then
        error("relCellPos is nil")
    end
    if currentMapData.Extents == nil then
        error("mapPos.Extents is nil")
    end

    local x = util.remap(
        relMeshPos.x,
        0.0, 1.0,
        currentMapData.Extents.Left,
        currentMapData.Extents.Right + 1
    )

    local y = util.remap(
        relMeshPos.y,
        0.0, 1.0,
        currentMapData.Extents.Bottom,
        currentMapData.Extents.Top + 1
    )

    return util.vector3(x, y, relMeshPos.z)
end


-- TODO: might be hot trash
local function mapPosToRelativeCellPos(currentMapData, worldPos)
    local bl = currentMapData.bounds.bottomLeft
    local br = currentMapData.bounds.bottomRight
    local tl = currentMapData.bounds.topLeft
    local tr = currentMapData.bounds.topRight

    -- Work in 2D
    local px, py = worldPos.x, worldPos.y

    local ax = bl.x
    local ay = bl.y

    local bx = br.x - bl.x
    local by = br.y - bl.y

    local cx = tl.x - bl.x
    local cy = tl.y - bl.y

    local dx = tr.x - tl.x - br.x + bl.x
    local dy = tr.y - tl.y - br.y + bl.y

    -- Solve for y using quadratic
    local A = dx * cy - dy * cx
    local B = dx * ay - dy * ax + bx * cy - by * cx + dy * px - dx * py
    local C = bx * ay - by * ax + by * px - bx * py

    local y
    if math.abs(A) < 1e-8 then
        -- Degenerate to linear
        y = -C / B
    else
        local disc = B * B - 4 * A * C
        if disc < 0 then
            error("point not on map quad")
        end
        local sqrtDisc = math.sqrt(disc)

        local y1 = (-B + sqrtDisc) / (2 * A)
        local y2 = (-B - sqrtDisc) / (2 * A)

        -- choose solution in [0,1]
        y = (y1 >= 0 and y1 <= 1) and y1 or y2
    end

    -- Solve for x
    local denom = bx + dx * y
    local x = (px - ax - cx * y) / denom

    return util.vector3(x, y, 0)
end



--- relativeMapPosToWorldPos turns a relative map position to a 3D world position,
--- which is the position on the map mesh.
--- @param currentMapData MeshAnnotatedMapData
--- @param relCellPos RelativeMeshPos
--- @return WorldSpacePos?
local function relativeMeshPosToAbsoluteMeshPos(currentMapData, relCellPos)
    if currentMapData == nil then
        error("no current map")
    end
    if currentMapData.object == nil then
        error("missing object")
    end
    if relCellPos == nil then
        error("relCellPos is nil")
    end
    if currentMapData.bounds == nil then
        error("currentMapData.bounds is nil")
    end
    -- interpolate along X at bottom and top edges
    local bottomPos = mutil.lerpVec3(currentMapData.bounds.bottomLeft, currentMapData.bounds.bottomRight, relCellPos.x)
    local topPos    = mutil.lerpVec3(currentMapData.bounds.topLeft, currentMapData.bounds.topRight, relCellPos.x)

    -- interpolate along Y between bottom and top
    local worldPos  = mutil.lerpVec3(bottomPos, topPos, relCellPos.y)

    local out       = util.vector3(worldPos.x, worldPos.y, currentMapData.bounds.bottomRight.z)

    --[[
    local inverse   = mapPosToRelativeCellPos(currentMapData, out)

    print("expected relCellPos: " ..
        tostring(relCellPos) .. "\ninput: " ..
        tostring(out) .. "\n actual relCellPos: " .. tostring(inverse))
    ]]
    return out
end




return {
    cellPosToRelativeMeshPos = cellPosToRelativeMeshPos,
    relativeMeshPosToAbsoluteMeshPos = relativeMeshPosToAbsoluteMeshPos,
    relativeMeshPosToCellPos = relativeMeshPosToCellPos,
    mapPosToRelativeCellPos = mapPosToRelativeCellPos,
}
