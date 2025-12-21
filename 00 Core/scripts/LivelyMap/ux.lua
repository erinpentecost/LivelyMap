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
local MOD_NAME        = require("scripts.LivelyMap.ns")
local mutil           = require("scripts.LivelyMap.mutil")
local putil           = require("scripts.LivelyMap.putil")
local core            = require("openmw.core")
local util            = require("openmw.util")
local pself           = require("openmw.self")
local aux_util        = require('openmw_aux.util')
local camera          = require("openmw.camera")
local ui              = require("openmw.ui")
local settings        = require("scripts.LivelyMap.settings")
local async           = require("openmw.async")
local interfaces      = require('openmw.interfaces')
local storage         = require('openmw.storage')
local heightData      = storage.globalSection(MOD_NAME .. "_heightData")
local h3cam           = require("scripts.LivelyMap.h3.cam")

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


local function realPosToViewportPos(pos, facingWorldDir)
    -- this works ok, but fails when the camera gets too close.
    if not currentMapData then
        error("no current map")
    end

    local cellPos = mutil.worldPosToCellPos(pos)
    local rel = putil.cellPosToRelativeMeshPos(currentMapData, cellPos)

    local mapWorldPos = putil.relativeMeshPosToAbsoluteMeshPos(currentMapData, rel)

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

    -- Extra calcs if we need facing
    local viewportFacing = nil
    if facingWorldDir then
        --print("facingWorldDir: " .. tostring(facingWorldDir))
        facingWorldDir = util.vector3(2000 * facingWorldDir.x, 2000 * facingWorldDir.y, 0)
        local relFacing = putil.cellPosToRelativeMeshPos(currentMapData, mutil.worldPosToCellPos(pos + facingWorldDir))

        local mapWorldFacingPos = putil.relativeMeshPosToAbsoluteMeshPos(currentMapData, relFacing)
        local s0 = h3cam.worldPosToViewportPos(mapWorldPos)
        local s1 = h3cam.worldPosToViewportPos(mapWorldFacingPos)
        if s0 and s1 then
            viewportFacing = (s1 - s0):normalize()
        end
    end


    return {
        viewportPos = h3cam.worldPosToViewportPos(mapWorldPos + parallaxWorldOffset),
        mapWorldPos = mapWorldPos,
        viewportFacing = viewportFacing,
    }
end


-- icons is a list of {widget, fn() worldPos}
local icons = {}

local function hideIcon(icon)
    if icon.onScreen then
        icon.onScreen = false
        icon.ref.onHide(icon.ref)
    end
end

local hoverBox = ui.create {
    name = 'hoverBox',
    type = ui.TYPE.Container,
    template = interfaces.MWUI.templates.boxSolid,
    props = {
        --relativePosition = util.vector2(0.5, 0.5),
        --size = util.vector2(30, 30),
        --anchor = util.vector2(0.5, 0.5),
        relativePosition = util.vector2(0.5, 0.9),
        anchor = util.vector2(0.5, 1),
        visible = false
    },
    content = ui.content { {
        name = "vflex",
        type = ui.TYPE.Flex,
        props = {
            arrange = ui.ALIGNMENT.Center,
            horizontal = false,
        },
        content = ui.content {},
    } }
}

local mainWindow = ui.create {
    name = "worldmaproot",
    layer = 'Windows',
    type = ui.TYPE.Widget,
    props = {
        size = ui.screenSize(),
        visible = false,
    },
    content = ui.content { hoverBox },
}

local function setHoverBoxContent(content)
    -- delete old items in hovercontent
    for _, old in ipairs(hoverBox.layout.content["vflex"].content) do
        if old.destroy then
            old:destroy()
        end
    end
    local temp = ui.content {}

    if content and #content > 0 then
        hoverBox.layout.props.visible = true

        for _, c in ipairs(content) do
            temp:add(c)
        end
    else
        hoverBox.layout.props.visible = false
    end

    hoverBox.layout.content["vflex"].content = temp

    hoverBox:update()
end

local function closeToCenter(viewportPos)
    local screenSize = ui.screenSize()
    local radius2 = (screenSize / 100):length2()
    if radius2 < 32 * 32 then
        radius2 = 32 * 32
    end
    return (viewportPos - (screenSize / 2)):length2() < radius2
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

    -- Track which icons we are hovering over.
    local hovering = {}

    -- Render all the icons.
    for i = #icons, 1, -1 do
        -- Remove if marked for removal.
        if icons[i].remove then
            table.remove(icons, i)
            goto continue
        end

        -- Get world position.
        local iPos = nil
        if type(icons[i].ref.pos) == "function" then
            iPos = icons[i].ref.pos()
        else
            iPos = icons[i].ref.pos
        end
        -- Get optional world facing vector.
        local iFacing = nil
        if icons[i].ref.facing then
            if type(icons[i].ref.facing) == "function" then
                iFacing = icons[i].ref.facing()
            else
                iFacing = icons[i].ref.facing
            end
        end

        if iPos then
            local pos = realPosToViewportPos(iPos, iFacing)
            if pos.viewportPos then
                icons[i].onScreen = true
                -- if the icon is hover-aware, get its info and
                -- embed the hover status in the pos table.
                if icons[i].ref.onHover and closeToCenter(pos.viewportPos) then
                    pos.hovering = true
                    table.insert(hovering, icons[i].ref.onHover(pos, icons[i].ref))
                end
                icons[i].ref.onDraw(pos, icons[i].ref)
            else
                hideIcon(icons[i])
            end
        else
            hideIcon(icons[i])
        end

        ::continue::
    end

    setHoverBoxContent(hovering)
end

local onMapMovedHandlers = {}
local onMapHiddenHandlers = {}

local function onMapMoved(data)
    print("onMapMoved" .. aux_util.deepToString(data, 3))
    currentMapData = data

    for _, fn in ipairs(onMapMovedHandlers) do
        fn(currentMapData)
    end

    if not data.swapped then
        -- need to steal some interface and also turn off pausing during it.
        -- this is because I want
        interfaces.UI.setMode('Interface', { windows = {} })
        --interfaces.UI.setPauseOnMode('Travel', false)
        mainWindow.layout.props.visible = true
        mainWindow:update()
    end
    renderIcons()
end

local function onMapHidden(data)
    print("onMapHidden" .. aux_util.deepToString(data, 3))
    for _, fn in ipairs(onMapHiddenHandlers) do
        fn(data)
    end

    if not data.swapped then
        interfaces.UI.setMode()
        mainWindow.layout.props.visible = false
        mainWindow:update()
    end
end

local lastCameraPos = nil
local function onUpdate(dt)
    if settingsChanged then
        renderIcons()
        settingsChanged = false
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
    mapData.startWorldPosition = {
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
        ref = icon,
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
