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
local MOD_NAME       = require("scripts.LivelyMap.ns")
local mutil          = require("scripts.LivelyMap.mutil")
local putil          = require("scripts.LivelyMap.putil")
local core           = require("openmw.core")
local util           = require("openmw.util")
local pself          = require("openmw.self")
local aux_util       = require('openmw_aux.util')
local myui           = require('scripts.LivelyMap.pcp.myui')
local camera         = require("openmw.camera")
local ui             = require("openmw.ui")
local settings       = require("scripts.LivelyMap.settings")
local async          = require("openmw.async")
local interfaces     = require('openmw.interfaces')
local storage        = require('openmw.storage')
local h3cam          = require("scripts.LivelyMap.h3.cam")
local overlapfinder  = require("scripts.LivelyMap.overlapfinder")

---@type MeshAnnotatedMapData?
local currentMapData = nil

local settingCache   = {
    psoUnlock       = settings.pso.psoUnlock,
    psoDepth        = settings.pso.psoDepth,
    psoPushdownOnly = settings.pso.psoPushdownOnly,
    debug           = settings.main.debug,
    palleteColor4   = settings.main.palleteColor4,
    palleteColor5   = settings.main.palleteColor5,
}
print("first run settings:" .. aux_util.deepToString(settingCache, 5))
settings.main.subscribe(async:callback(function(_, key)
    settingCache[key] = settings.main[key]
end))
settings.pso.subscribe(async:callback(function(_, key)
    settingCache[key] = settings.pso[key]
end))

---@class Icon
--- @field element any UI element.
--- @field pos fun(icon: Icon): util.vector3?
--- @field facing (fun(icon: Icon): util.vector2|util.vector3|nil)?
--- @field onDraw fun(icon: Icon, posData : ViewportData)
--- @field onHide fun(icon: Icon)
--- @field priority number? The higher the priority, the higher the layer.

---@class RegisteredIcon
--- @field onScreen boolean Exists so we don't call onHide every frame.
--- @field remove boolean Remove is used to signal deletion.
--- @field ref Icon
--- @field name string Matches the layout name.

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
    template = interfaces.MWUI.templates.boxTransparent,
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
    clickStartCenterCameraWorldPos = nil,
    thousandPixelsRight = nil,
    thousandPixelsUp = nil,
    dragThreshold = 10,
}
local function mapClicked(mouseEvent, data)
    print("click! " .. aux_util.deepToString(mouseEvent, 3) .. " worldspace: " .. tostring(mouseData.clickStartWorldPos))
    -- need to go from world pos to cam pos now
    interfaces.LivelyMapControls.trackToWorldPosition(mouseData.clickStartWorldPos, 1)
end
local function mapClickPress(mouseEvent, data)
    mouseData.clickStartViewportPos          = mouseEvent.position
    mouseData.clickStartWorldPos             = putil.viewportPosToRealPos(currentMapData, mouseEvent.position)
    mouseData.clickStartCenterCameraWorldPos = putil.viewportPosToRealPos(currentMapData, ui.screenSize() / 2)
end
local function mapClickRelease(mouseEvent, data)
    if (mouseEvent.position - mouseData.clickStartViewportPos):length2() < mouseData.dragThreshold then
        mapClicked(mouseEvent, data)
    end
    mouseData.clickStartViewportPos = nil
    mouseData.clickStartWorldPos = nil
    mouseData.dragging = false
    mouseData.thousandPixelsRight = nil
    mouseData.thousandPixelsUp = nil
end
local function mapDragStart(mouseEvent, data)
    -- oh I know the problem!
    -- the center of the camera is jumping to the drag anchor start immediately

    -- re-anchor drag start
    mouseData.clickStartViewportPos = mouseEvent.position
    mouseData.clickStartWorldPos    = putil.viewportPosToRealPos(currentMapData, mouseEvent.position)
    print("drag! " .. aux_util.deepToString(mouseEvent, 3) .. " worldspace: " .. tostring(mouseData.clickStartWorldPos))
    -- Snapshot projection basis
    local rightWorld              = putil.viewportPosToRealPos(
        currentMapData,
        mouseEvent.position + util.vector2(1000, 0)
    )

    local upWorld                 = putil.viewportPosToRealPos(
        currentMapData,
        mouseEvent.position + util.vector2(0, 1000)
    )

    mouseData.thousandPixelsRight = rightWorld - mouseData.clickStartWorldPos
    mouseData.thousandPixelsUp    = upWorld - mouseData.clickStartWorldPos
end
local function mapDragging(mouseEvent, data)
    local deltaViewport =
        mouseEvent.position - mouseData.clickStartViewportPos

    -- Convert viewport delta → world delta using frozen basis
    local deltaWorld =
        mouseData.thousandPixelsRight * (-deltaViewport.x) / 1000 +
        mouseData.thousandPixelsUp * (-deltaViewport.y) / 1000

    interfaces.LivelyMapControls.trackToWorldPosition(mouseData.clickStartCenterCameraWorldPos + deltaWorld, 0)
end
local function mapMouseMove(mouseEvent, data)
    if not mouseData.clickStartViewportPos then
        return
    end
    if (not mouseData.dragging) and (mouseEvent.position - mouseData.clickStartViewportPos):length2() >= mouseData.dragThreshold then
        mapDragStart(mouseEvent, data)
        mouseData.dragging = true
        -- the jump happens even if I return here
    end
    if not mouseData.dragging then
        return
    end

    mapDragging(mouseEvent, data)
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

local normalButtonColors = {
    default = settingCache.palleteColor5,
    over = mutil.lerpColor(settingCache.palleteColor5, util.color.rgb(1, 1, 1), 0.3),
    pressed = mutil.lerpColor(settingCache.palleteColor5, util.color.rgb(1, 1, 1), 0.5),
}
local psoButtonColors = {
    default = settingCache.palleteColor4,
    over = mutil.lerpColor(settingCache.palleteColor4, util.color.rgb(1, 1, 1), 0.3),
    pressed = mutil.lerpColor(settingCache.palleteColor4, util.color.rgb(1, 1, 1), 0.5),
}

local menuBarButtonSize = util.vector2(32, 32)

local function makeMenuButton(name, path, fn, buttonColors)
    local newButton = ui.create {}
    newButton.layout = myui.createButton(newButton,
        {
            name = name,
            type = ui.TYPE.Image,
            props = {
                anchor = util.vector2(0.5, 0.5),
                size = menuBarButtonSize,
                resource = ui.texture {
                    path = path,
                },
                color = buttonColors.default
            },
            userData = {}
        },
        function(layout, state)
            layout.props.color = buttonColors[state]
        end,
        fn, nil)
    newButton:update()
    return newButton
end

local function markerButtonFn()
    print("markerbutton clicked")
    --- position is pretty accurate so converting it to a string
    --- is basically random
    local newID = "custom_" .. tostring(math.floor(pself.position.x)) .. "_" .. tostring(math.floor(pself.position.y))
    interfaces.LivelyMapMarker.editMarkerWindow({ id = newID })
end
local newMarkerButton = makeMenuButton("markerButton", "textures/LivelyMap/marker-button.png", markerButtonFn,
    normalButtonColors)
local function journeyButtonFn()
    print("journeybutton clicked")
    interfaces.LivelyMapJourneyIcons.toggleJourney()
end
local journeyButton = makeMenuButton("journeyButton", "textures/LivelyMap/journey-button.png", journeyButtonFn,
    normalButtonColors)

local psoReduceDepthButton = makeMenuButton("psoReduceDepthButton", "textures/LivelyMap/minus-button.png",
    function()
        settings.pso.section:set("psoDepth", math.max(0, settingCache.psoDepth - 1))
    end,
    psoButtonColors
)
local psoIncreaseDepthButton = makeMenuButton("psoIncreaseDepthButton", "textures/LivelyMap/plus-button.png",
    function()
        settings.pso.section:set("psoDepth", math.min(300, settingCache.psoDepth + 1))
    end,
    psoButtonColors
)
local psoTogglePushdownButton = makeMenuButton("psoTogglePushdownButton", "textures/LivelyMap/pushdown-button.png",
    function()
        settings.pso.section:set("psoPushdownOnly", not settingCache.psoPushdownOnly)
    end,
    psoButtonColors
)

local psoMenuButtons = ui.create {
    name = 'psoMenuButtons',
    type = ui.TYPE.Flex,
    props = {
        horizontal = true,
    },
    content = ui.content {
        myui.padWidget(10, 10),
        psoReduceDepthButton,
        psoIncreaseDepthButton,
        psoTogglePushdownButton,
    }
}

local menuBar = ui.create {
    name = 'menuBar',
    type = ui.TYPE.Container,
    template = interfaces.MWUI.templates.boxTransparent,
    props = {
        --relativePosition = util.vector2(0.5, 0.5),
        --size = util.vector2(200, 50),
        --anchor = util.vector2(0.5, 0.5),
        relativePosition = util.vector2(0.5, 0.0),
        anchor = util.vector2(0.5, 0),
        visible = true,
        propagateEvents = false,
    },
    content = ui.content {
        {
            name = 'mainV',
            type = ui.TYPE.Flex,
            props = {
                horizontal = true,
            },
            content = ui.content {
                newMarkerButton,
                myui.padWidget(10, 10),
                journeyButton,
                settingCache.psoUnlock and psoMenuButtons or nil,
            }
        }
    }
}

settings.pso.subscribe(async:callback(function(_, key)
    if key == "psoUnlock" then
        print("psoUnlock changed")
        local idx = menuBar.layout.content["mainV"].content:indexOf(psoMenuButtons)
        if settings.pso[key] and not idx then
            print("adding pso buttons. idx=" .. tostring(idx))
            menuBar.layout.content["mainV"].content:add(psoMenuButtons)
            menuBar:update()
        elseif (not settings.pso[key]) and idx then
            print("removing pso buttons. idx=" .. tostring(idx))
            menuBar.layout.content["mainV"].content[idx] = nil
            menuBar:update()
        end
    end
end))

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
    content = ui.content { iconContainer, hoverBox, menuBar },
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

local function purgeRemovedIcons()
    --- check if remove is pending
    local doRemoval = false
    for _, icon in ipairs(icons) do
        if icon.remove then
            doRemoval = true
            break
        end
    end

    if not doRemoval then
        return
    end

    local remainingIcons = {}
    local remainingContent = {}

    for _, icon in ipairs(icons) do
        if not icon.remove then
            table.insert(remainingIcons, icon)
            table.insert(remainingContent, icon.ref.element)
            -- icon is responsible for destroying the UI element
        elseif settingCache.debug then
            print("Removing icon '" .. icon.name .. "'.")
        end
    end

    icons = remainingIcons
    iconContainer.layout.content = ui.content(remainingContent)
    if #remainingIcons ~= #remainingContent then
        error("mismatch between icons list and icons container content")
    end
end


---@param icon RegisteredIcon
---@return RectExtent
local function getIconExtent(icon)
    -- assumes anchor is 0.5,0.5
    local halfSize = icon.ref.element.layout.props.size / 2
    return {
        topLeft = icon.ref.element.layout.props.position + halfSize,
        bottomRight = icon.ref.element.layout.props.position - halfSize,
    }
end

local function renderIcons()
    -- If there is no map, hide all icons.
    if currentMapData == nil then
        for _, icon in ipairs(icons) do
            hideIcon(icon)
        end
        return
    end

    purgeRemovedIcons()

    local screenSize = ui.screenSize()

    local collisionFinder = overlapfinder.NewOverlapFinder(getIconExtent)

    -- Render all the icons.
    for _, icon in ipairs(icons) do
        -- Get world position.
        local iPos = icon.ref.pos(icon.ref)
        -- Get optional world facing vector.
        local iFacing = icon.ref.facing and icon.ref.facing(icon.ref) or nil

        if iPos then
            local pos = putil.realPosToViewportPos(currentMapData, settingCache, iPos, iFacing)
            if pos and pos.viewportPos then
                if pos.viewportPos.pos and pos.viewportPos.onScreen then
                    icon.onScreen = true
                    icon.ref.onDraw(icon.ref, pos)
                    collisionFinder:AddElement(icon)
                    goto continue
                elseif pos.viewportPos.pos and icon.ref.element.layout.props.size then
                    -- is the edge visible?
                    local halfBox = icon.ref.element.layout.props.size / 2
                    local min = pos.viewportPos.pos - halfBox
                    local max = pos.viewportPos.pos + halfBox

                    if max.x >= 0 and max.y >= 0 and
                        min.x <= screenSize.x and min.y <= screenSize.y then
                        icon.onScreen = true
                        icon.ref.onDraw(icon.ref, pos)
                        collisionFinder:AddElement(icon)
                        goto continue
                    end
                end
            end
        end
        hideIcon(icon)
        :: continue ::
    end


    --- do we need to combine any?
    ---@type RegisteredIcon[][]
    local overlaps = collisionFinder:GetOverlappingSubsets()
    for _, subset in ipairs(overlaps) do
        if #subset > 1 then
            -- this is a set of atleast 2
            print("Colliding icons: ")
            for _, elem in ipairs(subset) do
                print("- " .. elem.name)
            end
        end
    end

    iconContainer:update()
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
        interfaces.UI.addMode('Interface', { windows = {} })
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
        interfaces.UI.removeMode('Interface')
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

---@param open boolean? Nil to toggle. Otherwise, boolean indicating desired state.
local function toggleMap(open)
    if open == nil then
        open = currentMapData == nil
    end
    if open and currentMapData == nil then
        summonMap()
    elseif (not open) and (currentMapData ~= nil) then
        core.sendGlobalEvent(MOD_NAME .. "onHideMap", { player = pself })
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
    icon.element.layout.name = name

    if settingCache.debug then
        print("Registering icon '" .. name .. "': " .. aux_util.deepToString(icon, 4))
    end

    --- Determine where to insert the icon
    if icon.priority == nil then
        icon.priority = 0
    elseif type(icon.priority) ~= "number" then
        error("icon.priority must be a number")
    end
    local insertIndex = mutil.binarySearchFirst(icons, function(p) return p.ref.priority > icon.priority end)

    if settingCache.debug then
        print("Inserted at index " .. tostring(insertIndex) .. " of " .. tostring(#icons) .. ".")
    end

    table.insert(icons, insertIndex, {
        -- onScreen exists so we don't call onHide every frame.
        onScreen = false,
        -- remove is used to signal deletion
        remove = false,
        ref = icon,
        name = name,
    })

    icon.onHide(icon)
    iconContainer.layout.content:insert(insertIndex, icon.element)
    iconContainer:update()

    return name
end


local function addHandler(fn, list)
    if type(fn) ~= "function" then
        error("addHandler fn must be a function, not a " .. type(fn))
    end
    table.insert(list, fn)
end

local function getIcon(name)
    for _, icon in ipairs(icons) do
        if icon.name == name then
            return icon
        end
    end
    return nil
end

return {
    interfaceName = MOD_NAME .. "Draw",
    interface = {
        version = 1,
        registerIcon = registerIcon,
        getIcon = getIcon,
        setHoverBoxContent = setHoverBoxContent,
        onMapMoved = function(fn)
            return addHandler(fn, onMapMovedHandlers)
        end,
        onMapHidden = function(fn)
            return addHandler(fn, onMapHiddenHandlers)
        end,
        toggleMap = toggleMap,
    },
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
        [MOD_NAME .. "onMapHidden"] = onMapHidden,
    },
    engineHandlers = {
        onUpdate = onUpdate,
    }
}
