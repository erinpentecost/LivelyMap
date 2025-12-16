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
local MOD_NAME        = require("scripts.LivelyMap.ns")
local mutil           = require("scripts.LivelyMap.mutil")
local core            = require("openmw.core")
local util            = require("openmw.util")
local pself           = require("openmw.self")
local aux_util        = require('openmw_aux.util')
local camera          = require("openmw.camera")
local ui              = require("openmw.ui")
local settings        = require("scripts.LivelyMap.settings")
local async           = require("openmw.async")

local storage         = require('openmw.storage')
local heightData      = storage.globalSection(MOD_NAME .. "_heightData")

local currentMapData  = nil

-- psoDepth determines how much to offset icons on the map.
local settingCache    = {
    psoDepth        = settings.psoDepth,
    psoPushdownOnly = settings.psoPushdownOnly,
    psoUnlock       = settings.psoUnlock,
}
local settingsChanged = false
settings.subscribe(async:callback(function(_, key)
    settingCache[key] = settings[key]
    settingsChanged = true
end))


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
    local x = util.remap(cellPos.x, currentMapData.Extents.Left, currentMapData.Extents.Right + 1, 0.0, 1.0)
    local y = util.remap(cellPos.y, currentMapData.Extents.Bottom, currentMapData.Extents.Top + 1, 0.0, 1.0)
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

    return util.vector3(worldPos.x, worldPos.y, currentMapData.bounds.bottomRight.z)
end


local function worldPosToViewportPos(worldPos)
    -- this function works perfectly
    local viewportPos = camera.worldToViewportVector(worldPos)
    local screenSize = ui.screenSize()

    local validX = viewportPos.x > 0 and viewportPos.x < screenSize.x
    local validY = viewportPos.y > 0 and viewportPos.y < screenSize.y
    local withinViewDistance = viewportPos.z <= camera.getViewDistance()

    if not validX or not validY or not withinViewDistance then return end

    if isObjectBehindCamera(worldPos) then return end

    return util.vector2(viewportPos.x, viewportPos.y)
end


local function realPosToViewportPos(pos)
    -- this works ok, but fails when the camera gets too close.
    if not currentMapData then
        error("no current map")
    end

    local cellPos = mutil.worldPosToCellPos(pos)
    local rel = relativeCellPos(cellPos)

    local mapWorldPos = relativeCellPosToMapPos(rel)

    -- POM: Calculate vertical offset so the icon appears glued
    -- to the surface of the map, which has been distorted according
    -- to the parallax shader.
    local maxHeight = heightData:get("MaxHeight")
    local height = util.clamp(rel.z * mutil.CELL_SIZE, 0, maxHeight)
    local heightMax = 0.5
    if settingCache.psoPushdownOnly then
        heightMax = 1.0
    end
    local heightRatio = heightMax - (height / maxHeight)
    local camPos = camera.getPosition()
    local viewDir = (camPos - mapWorldPos):normalize()
    --local safeZ = math.max(math.abs(viewDir.z), 0.1)
    local safeZ = 1
    local parallaxWorldOffset =
        util.vector3(
            viewDir.x / safeZ,
            viewDir.y / safeZ,
            0
        ) * (settingCache.psoDepth * heightRatio)
    -- POM Distance fade
    local maxPOMDistance = 1000
    local dist = (camPos - mapWorldPos):length()
    local fade = 1.0 - util.clamp(dist / maxPOMDistance, 0, 1)

    parallaxWorldOffset = parallaxWorldOffset * fade

    return {
        viewportPos = worldPosToViewportPos(mapWorldPos + parallaxWorldOffset),
        mapWorldPos = mapWorldPos,
    }
end


-- icons is a list of {widget, fn() worldPos}
local icons = {}

local function hideIcon(icon)
    if icon.onScreen then
        icon.onScreen = false
        icon.onHide()
    end
end

local function renderIcons()
    -- If there is no map, hide all icons.
    if currentMapData == nil then
        for i = #icons, 1, -1 do
            if icons[i].remove then
                table.remove(icons, i)
            else
                hideIcon(icons[i])
            end
        end
        return
    end

    -- Render all the icons.
    for i = #icons, 1, -1 do
        -- Remove if marked for removal.
        if icons[i].remove then
            table.remove(icons, i)
            goto continue
        end

        -- Get world position.
        local iPos = nil
        if type(icons[i].pos) == "function" then
            iPos = icons[i].pos()
        else
            iPos = icons[i].pos
        end

        if iPos then
            local pos = realPosToViewportPos(iPos)
            if pos.viewportPos then
                icons[i].onScreen = true
                icons[i].onDraw(pos)
            else
                hideIcon(icons[i])
            end
        else
            hideIcon(icons[i])
        end

        ::continue::
    end
end

local onMapMovedHandlers = {}
local onMapHiddenHandlers = {}

local function onMapMoved(data)
    print("onMapMoved" .. aux_util.deepToString(data, 3))
    currentMapData = data

    for _, fn in ipairs(onMapMovedHandlers) do
        fn(currentMapData)
    end

    renderIcons()
end

local function onMapHidden(data)
    print("onMapHidden" .. aux_util.deepToString(data, 3))
    for _, fn in ipairs(onMapHiddenHandlers) do
        fn(data)
    end
end

local lastCameraPos = nil
local function onUpdate(dt)
    if settingsChanged then
        renderIcons()
        settingsChanged = false
        return
    end
    if dt <= 0 then
        return
    end
    if lastCameraPos == nil then
        lastCameraPos = camera.getPosition()
        renderIcons()
    else
        local curPos = camera.getPosition()
        if lastCameraPos ~= curPos then
            lastCameraPos = curPos
            renderIcons()
        end
    end
end

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


local function registerIcon(icon)
    if not icon then
        error("registerIcon icon is nil")
    end
    if not icon.pos then
        error("registerIcon icon.pos is nil: " .. aux_util.deepToString(icon, 3))
    end
    if not icon.onDraw then
        error("registerIcon icon.onDraw is nil: " .. aux_util.deepToString(icon, 3))
    end
    if not icon.onHide then
        error("registerIcon icon.onHide is nil: " .. aux_util.deepToString(icon, 3))
    end
    table.insert(icons, {
        -- onScreen exists so we don't call onHide every frame.
        onScreen = false,
        -- remove is used to signal deletion
        remove = false,
        pos = icon.pos,
        onDraw = icon.onDraw,
        onHide = icon.onHide,
    })
end


local function addHandler(fn, list)
    if type(fn) ~= "function" then
        error("addHandler fn must be a function, not a " .. type(fn))
    end
    table.insert(list, fn)
end

return {
    interfaceName = MOD_NAME .. "Draw",
    interface = {
        version = 1,
        registerIcon = registerIcon,
        onMapMoved = function(fn)
            return addHandler(fn, onMapMovedHandlers)
        end,
        onMapHidden = function(fn)
            return addHandler(fn, onMapHiddenHandlers)
        end
    },
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
        [MOD_NAME .. "onMapHidden"] = onMapHidden,
    },
    engineHandlers = {
        onUpdate = onUpdate,
        onConsoleCommand = onConsoleCommand,
    }
}
