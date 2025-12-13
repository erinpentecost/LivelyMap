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
local MOD_NAME   = require("scripts.LivelyMap.ns")
local mutil      = require("scripts.LivelyMap.mutil")
local core       = require("openmw.core")
local util       = require("openmw.util")
local pself      = require("openmw.self")
local aux_util   = require('openmw_aux.util')
local camera     = require("openmw.camera")
local ui         = require("openmw.ui")
local compass    = require("scripts.LivelyMap.compass")
local settings   = require("scripts.LivelyMap.settings")
local async      = require("openmw.async")

local storage    = require('openmw.storage')
local heightData = storage.globalSection(MOD_NAME .. "_heightData")

local pom_depth  = settings.pomDepth

settings.subscribe(async:callback(function(_, setting)
    if setting == "pomDepth" then
        pom_depth = settings.pomDepth
    end
end))

local function summonMap(id)
    local mapData
    if id == "" or id == nil then
        mapData = mutil.getClosestMap(pself.cell.gridX, pself.cell.gridY)
    else
        mapData = mutil.getMap(id)
    end

    mapData.cellID = pself.cell.id
    mapData.player = pself
    mapData.position = {
        x = pself.position.x,
        y = pself.position.y,
        z = pself.position.z,
    }
    core.sendGlobalEvent(MOD_NAME .. "onShowMap", mapData)
end

local function splitString(str)
    local out = {}
    for item in str:gmatch("([^,%s]+)") do
        table.insert(out, item)
    end
    return out
end

local function onConsoleCommand(mode, command, selectedObject)
    local function getSuffixForCmd(prefix)
        if string.sub(command:lower(), 1, string.len(prefix)) == prefix then
            return string.sub(command, string.len(prefix) + 1)
        else
            return nil
        end
    end
    local showMap = getSuffixForCmd("lua map")

    if showMap ~= nil then
        local id = splitString(showMap)
        print("Show Map: " .. aux_util.deepToString(id, 3))

        if #id == 0 then
            id = nil
            summonMap(nil)
        else
            summonMap(id[1])
        end
    end
end

local function isObjectBehindCamera(worldPos)
    -- this function works perfectly
    local cameraPos = camera.getPosition()
    local cameraForward = util.transform.identity
        * util.transform.rotateZ(camera.getYaw())
        * util.vector3(0, 1, 0)

    -- Direction vector from camera to object
    local toObject = worldPos - cameraPos

    -- Normalize both vectors
    cameraForward = cameraForward:normalize()
    toObject = toObject:normalize()

    -- Calculate the dot product
    local dotProduct = cameraForward:dot(toObject)

    -- If the dot product is negative, the object is behind the camera
    return dotProduct < 0
end

local currentMapData = nil

-- relativeCellPos return mapPos, but shifted by the current map Extents
-- so the bottom left becomes 0,0 and top right becomes 1,1.
local function relativeCellPos(cellPos)
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
    local x = util.remap(cellPos.x, currentMapData.Extents.Left, currentMapData.Extents.Right, 0.0, 1.0)
    local y = util.remap(cellPos.y, currentMapData.Extents.Bottom, currentMapData.Extents.Top, 0.0, 1.0)
    return util.vector3(x, y, cellPos.z)
end

-- relativeMapPosToWorldPos turns a relative map position to a 3D world position,
-- which is the position on the map mesh.
local function relativeCellPosToMapPos(relCellPos)
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

    local maxHeight = heightData:get("MaxHeight")

    -- this isn't right, because the strength of the POM (parallax occlusion mapping)
    -- effect is diminished at large camera distances from this point.
    -- but this would actually do the inverse: dropping the icon lower
    -- as our angle becomes more oblique.
    --local z         = currentMapData.bounds.bottomRight.z +
    --    util.remap(relCellPos.z * mutil.CELL_SIZE, 0, maxHeight, 0, pom_depth) - pom_depth


    return util.vector3(worldPos.x, worldPos.y, currentMapData.bounds.bottomRight.z)
end


local function worldPosToViewportPos(worldPos, screenYOffset)
    -- this function works perfectly
    local viewportPos = camera.worldToViewportVector(worldPos)
    local screenSize = ui.screenSize()

    local validX = viewportPos.x > 0 and viewportPos.x < screenSize.x
    local validY = viewportPos.y > 0 and viewportPos.y < screenSize.y
    local withinViewDistance = viewportPos.z <= camera.getViewDistance()

    if not validX or not validY or not withinViewDistance then return end

    if isObjectBehindCamera(worldPos) then return end

    return util.vector2(viewportPos.x, viewportPos.y + screenYOffset)
end

local function realPosToViewportPos(pos)
    if currentMapData == nil then
        error("no current map")
    end

    local cellPos = mutil.worldPosToCellPos(pos)
    local rel = relativeCellPos(cellPos)

    local mapWorldPos = relativeCellPosToMapPos(rel)

    -- y offset is not accurate at all

    local maxHeight = heightData:get("MaxHeight")
    local heightRatio =
        1.0 - util.clamp((rel.z * mutil.CELL_SIZE) / maxHeight, 0, 1)

    local camPos = camera.getPosition()
    local viewDir = (camPos - mapWorldPos):normalize()

    local normal = util.vector3(0, 0, 1)
    local angleFactor = util.clamp(viewDir:dot(normal), 0, 1)

    local screenYOffset = pom_depth * heightRatio * angleFactor

    local maxPOMDistance = 1000
    local dist = (camPos - mapWorldPos):length()
    local fade = 1.0 - util.clamp(dist / maxPOMDistance, 0, 1)
    local screenYOffsetWithFade = screenYOffset * fade

    print("heightRatio: " ..
        heightRatio ..
        ", screenYOffset: " ..
        screenYOffset .. ", angleFactor: " .. angleFactor .. ", screenYOffsetWithFade: " .. screenYOffsetWithFade)

    return worldPosToViewportPos(mapWorldPos, screenYOffsetWithFade)
end


local function onMapMoved(data)
    print("onMapMoved" .. aux_util.deepToString(data, 3))
    currentMapData = data
end

-- icons is a list of {widget, fn() worldPos}
local icons = {
    {
        widget = compass,
        pos = function()
            --return mutil.worldPosToCellPos(pself.position)
            return pself.position
        end
    },
}

local function onUpdate(dt)
    if dt <= 0 then
        return
    end
    -- todo: optimize
    if currentMapData == nil then
        for _, icon in ipairs(icons) do
            icon.widget.layout.props.visible = false
            icon.widget:update()
        end
        return
    end

    for _, icon in ipairs(icons) do
        --local pos = cellPosToViewportPosition(icon.pos())
        local pos = realPosToViewportPos(icon.pos())
        if pos then
            --[[if pos ~= icon.widget.layout.props.position then
                print(pos)
                end]]
            icon.widget.layout.props.visible = true
            icon.widget.layout.props.position = pos
            icon.widget:update()
        else
            icon.widget.layout.props.visible = false
            icon.widget:update()
        end
    end
end

return {
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
    },
    engineHandlers = {
        onUpdate = onUpdate,
        onConsoleCommand = onConsoleCommand,
    }
}
