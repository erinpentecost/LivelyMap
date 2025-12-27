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

---@type MeshAnnotatedMapData?
local currentMapData  = nil

-- psoDepth determines how much to offset icons on the map.
local settingCache    = {
    psoDepth        = settings.psoDepth,
    psoPushdownOnly = settings.psoPushdownOnly,
}
local settingsChanged = false
settings.subscribe(async:callback(function(_, key)
    settingCache[key] = settings[key]
    settingsChanged = true
end))

---@class Icon
--- @field element any UI element.
--- @field pos fun(): util.vector3?
--- @field facing (fun(): util.vector2|util.vector3|nil)?
--- @field onDraw fun(posData : ViewportData, icon: Icon)
--- @field onHide fun(icon: Icon)

---@class RegisteredIcon
--- @field onScreen boolean Exists so we don't call onHide every frame.
--- @field remove boolean Remove is used to signal deletion.
--- @field ref Icon
--- @name string

---@type RegisteredIcon[]
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
        --size = util.vector2(200, 50),
        --anchor = util.vector2(0.5, 0.5),
        relativePosition = util.vector2(0.5, 0.9),
        anchor = util.vector2(0.5, 1),
        visible = false
    },
    content = ui.content {}
}

local mouseData = {
    dragging = false,
    clickStartViewportPos = nil,
    clickStartWorldPos = nil,
    pixelRight = nil,
    pixelUp = nil,
}
local function mapClicked(mouseEvent, data)
    print("click! " .. aux_util.deepToString(mouseEvent, 3) .. " worldspace: " .. tostring(mouseData.clickStartWorldPos))
    -- need to go from world pos to cam pos now
    interfaces.LivelyMapControls.trackToWorldPosition(mouseData.clickStartWorldPos, 1)
end
local function mapClickPress(mouseEvent, data)
    mouseData.clickStartViewportPos = mouseEvent.position
    mouseData.clickStartWorldPos    = putil.viewportPosToRealPos(currentMapData, mouseEvent.position)
    -- Snapshot projection basis
    local rightWorld                = putil.viewportPosToRealPos(
        currentMapData,
        mouseEvent.position + util.vector2(1000, 0)
    )

    local upWorld                   = putil.viewportPosToRealPos(
        currentMapData,
        mouseEvent.position + util.vector2(0, 1000)
    )

    mouseData.thousandPixelsRight   = rightWorld - mouseData.clickStartWorldPos
    mouseData.thousandPixelsUp      = upWorld - mouseData.clickStartWorldPos
end
local function mapClickRelease(mouseEvent, data)
    if (mouseEvent.position - mouseData.clickStartViewportPos):length2() < 30 then
        mapClicked(mouseEvent, data)
    end
    mouseData.clickStartViewportPos = nil
    mouseData.clickStartWorldPos = nil
    mouseData.dragging = false
end
local function mapMouseMove(mouseEvent, data)
    if not mouseData.clickStartViewportPos then
        return
    end
    if (not mouseData.dragging) and (mouseEvent.position - mouseData.clickStartViewportPos):length2() >= 30 then
        mouseData.dragging = true
    end
    if not mouseData.dragging then
        return
    end

    local deltaViewport =
        mouseEvent.position - mouseData.clickStartViewportPos

    -- Convert viewport delta → world delta using frozen basis
    local deltaWorld =
        mouseData.thousandPixelsRight * (-deltaViewport.x) / 1000 +
        mouseData.thousandPixelsUp * (-deltaViewport.y) / 1000

    interfaces.LivelyMapControls.trackToWorldPosition(mouseData.clickStartWorldPos + deltaWorld, 0)
end


local iconContainer = ui.create {
    name = "icons",
    --layer = 'Windows',
    type = ui.TYPE.Widget,
    props = {
        size = ui.screenSize(),
    },
    events = {
    },
    content = ui.content {},
}

local mainWindow = ui.create {
    name = "worldmaproot",
    layer = 'Windows',
    type = ui.TYPE.Widget,
    props = {
        size = ui.screenSize(),
        visible = false,
    },
    events = {
        mousePress = async:callback(mapClickPress),
        mouseRelease = async:callback(mapClickRelease),
        mouseMove = async:callback(mapMouseMove),
    },
    content = ui.content { iconContainer, hoverBox },
}

--- Change hover box content.
---@param layout any UI element or layout. Set to empty or nil to clear the hover box.
local function setHoverBoxContent(layout)
    if layout then
        hoverBox.layout.content = ui.content { layout }
        hoverBox.layout.props.visible = true
    else
        hoverBox.layout.content = ui.content {}
        hoverBox.layout.props.visible = false
    end
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
                --iconContainer.layout.content[icons[i].name]:destroy()
                table.remove(icons, i)
            else
                hideIcon(icons[i])
            end
        end
        return
    end

    local screenSize = ui.screenSize()

    -- Render all the icons.
    for i = #icons, 1, -1 do
        -- Remove if marked for removal.
        if icons[i].remove then
            table.remove(icons, i)
            goto continue
        end

        -- Get world position.
        local iPos = icons[i].ref.pos()
        -- Get optional world facing vector.
        local iFacing = icons[i].ref.facing and icons[i].ref.facing() or nil

        if iPos then
            local pos = putil.realPosToViewportPos(currentMapData, settingCache, iPos, iFacing)
            if pos and pos.viewportPos then
                if pos.viewportPos.pos and pos.viewportPos.onScreen then
                    icons[i].onScreen = true
                    icons[i].ref.onDraw(pos, icons[i].ref)
                elseif pos.viewportPos.pos and icons[i].ref.element.layout.props.size then
                    -- is the edge visible?
                    local halfBox = icons[i].ref.element.layout.props.size / 2
                    local min = pos.viewportPos.pos - halfBox
                    local max = pos.viewportPos.pos + halfBox

                    if max.x >= 0 and max.y >= 0 and
                        min.x <= screenSize.x and min.y <= screenSize.y then
                        icons[i].onScreen = true
                        icons[i].ref.onDraw(pos, icons[i].ref)
                    else
                        hideIcon(icons[i])
                    end
                else
                    hideIcon(icons[i])
                end
            else
                hideIcon(icons[i])
            end
        else
            hideIcon(icons[i])
        end

        ::continue::
    end


    mainWindow:update()

    --print("iconContainer: " .. aux_util.deepToString(iconContainer.layout.props))


    -- debugging
    --[[
    local screenCenter = ui.screenSize() / 2
    local cameraFocusPos = putil.viewportPosToRealPos(currentMapData, screenCenter)
    if cameraFocusPos then
        local recalced = putil.realPosToViewportPos(currentMapData, settingCache, cameraFocusPos)
        if recalced then
            print("viewportPosToRealPos(mapData, " .. tostring(screenCenter) .. "): " ..
                tostring(cameraFocusPos) ..
                "\n realPosToViewportPos(mapData, " ..
                tostring(settingCache) .. ", " .. tostring(cameraFocusPos) .. "): " .. aux_util.deepToString(recalced, 3))
        end
    end
    --]]
end

local onMapMovedHandlers = {}
local onMapHiddenHandlers = {}

---@param data MeshAnnotatedMapData
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
    -- TODO: maybe hide icons?
    currentMapData = nil
end

--local lastCameraPos = nil
local function onUpdate(dt)
    if currentMapData == nil then
        return
    end
    renderIcons()
    --[[if settingsChanged then
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
        end]]
    --- TODO: icons aren't being drawn on the first frame of map spawn,
    --- probably because the camera is not in the right spot.
end

local function summonMap(id)
    local mapData
    if id == "" or id == nil then
        mapData = mutil.getClosestMap(pself.cell.gridX, pself.cell.gridY)
    else
        mapData = mutil.getMap(id)
    end

    local showData = mutil.shallowMerge(mapData, {
        cellID = pself.cell.id,
        player = pself,
        startWorldPosition = {
            x = pself.position.x,
            y = pself.position.y,
            z = pself.position.z,
        },
    })
    core.sendGlobalEvent(MOD_NAME .. "onShowMap", showData)
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

local nextName = 0
local function registerIcon(icon)
    if not icon then
        error("registerIcon icon is nil")
    end
    if not icon.element or type(icon.element) ~= "userdata" then
        error("registerIcon icon.element is: " .. aux_util.deepToString(icon, 3) .. ", expected UI element.")
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

    nextName = nextName + 1
    local name = "icon_" .. tostring(nextName)
    table.insert(icons, {
        -- onScreen exists so we don't call onHide every frame.
        onScreen = false,
        -- remove is used to signal deletion
        remove = false,
        ref = icon,
        name = name,
    })
    icon.element.layout.name = name
    icon.onHide()
    if icon.front then
        iconContainer.layout.content:insert(1, icon.element)
    else
        iconContainer.layout.content:add(icon.element)
    end
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
        setHoverBoxContent = setHoverBoxContent,
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
