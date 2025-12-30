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
local MOD_NAME     = require("scripts.LivelyMap.ns")
local storage      = require('openmw.storage')
local interfaces   = require('openmw.interfaces')
local ui           = require('openmw.ui')
local async        = require("openmw.async")
local util         = require('openmw.util')
local settings     = require("scripts.LivelyMap.settings")
local mutil        = require("scripts.LivelyMap.mutil")
local iutil        = require("scripts.LivelyMap.icons.iutil")
local vfs          = require('openmw.vfs')
local aux_util     = require('openmw_aux.util')
local myui         = require('scripts.LivelyMap.pcp.myui')
local core         = require("openmw.core")
local pself        = require("openmw.self")
local localization = core.l10n(MOD_NAME)

local settingCache = {
    palleteColor1 = settings.palleteColor1,
    palleteColor2 = settings.palleteColor2,
    palleteColor3 = settings.palleteColor3,
    palleteColor4 = settings.palleteColor4,
    palleteColor5 = settings.palleteColor5,
    debug = settings.debug,
}
settings.subscribe(async:callback(function(_, key)
    settingCache[key] = settings[key]
end))

local markerData = storage.playerSection(MOD_NAME .. "_markerData")
markerData:setLifeTime(storage.LIFE_TIME.Persistent)

local baseSize = util.vector2(32, 32)

---@class MarkerData
---@field id string Unique internal ID.
---@field hidden boolean Basically soft-delete.
---@field worldPos util.vector3
---@field iconName string Basename of the stamp.
---@field note string This appears in the hover box. Empty is valid!
---@field color number This corresponds to the pallete color number.

---@class RegisteredMarker : Icon
---@field marker MarkerData

---@type {[string]: RegisteredMarker}
local markerIcons = {}

local function resolveColor(colorID)
    if colorID == 1 then
        return settingCache.palleteColor1
    elseif colorID == 2 then
        return settingCache.palleteColor2
    elseif colorID == 3 then
        return settingCache.palleteColor3
    elseif colorID == 4 then
        return settingCache.palleteColor4
    elseif colorID == 5 then
        return settingCache.palleteColor5
    else
        error("unknown color ID " .. colorID)
    end
end

---@class StampID
---@field idx number
---@field basename string

local stampPathData = {
    ---@type string[]
    orderedFullPaths = {},
    ---@type {[string]: string}
    basenameToFullPathMap = {},
    ---@type {[string]: StampID}
    fullPathToIDMap = {}
}
--- stable list of all available stamps
local function stampList()
    for stampPath in vfs.pathsWithPrefix("textures\\LivelyMap\\stamps") do
        table.insert(stampPathData.orderedFullPaths, stampPath)
        local baseName = string.match(stampPath, '(%a+)[.]')
        stampPathData.basenameToFullPathMap[baseName] = stampPath
        stampPathData.fullPathToIDMap[stampPath] = {
            idx = #stampPathData.orderedFullPaths,
            basename = baseName,
        }
    end
end
stampList()
local function resolveStampFullPath(id)
    if type(id) == "number" then
        local lookup = ((id - 1) % #stampPathData.orderedFullPaths) + 1
        return stampPathData.orderedFullPaths[lookup]
    end
    if type(id) == "string" then
        return stampPathData.basenameToFullPathMap[id]
    end
end
local function stampIndexToBaseName(idx)
    print("stampIndexToBaseName(" .. tostring(idx) .. "): ")
    return stampPathData.fullPathToIDMap[resolveStampFullPath(idx)].basename
end

---@param data MarkerData
---@return boolean
local function validateMarker(data)
    if not data then
        return false
    end
    return data.color and data.iconName and data.id and data.note and data.worldPos and (type(data.id) == "string") and
        true or false
end

---@param data MarkerData
local function registerMarkerIcon(data)
    if not validateMarker(data) then
        error("marker data invalid: " .. aux_util.deepToString(data, 3))
    end
    print("registerMarkerIcon: " .. aux_util.deepToString(data, 3))
    local element = ui.create {
        type = ui.TYPE.Image,
        props = {
            visible = true,
            position = util.vector2(100, 100),
            anchor = util.vector2(0.5, 0.5),
            size = baseSize,
            resource = ui.texture {
                path = resolveStampFullPath(data.iconName)
            },
            color = resolveColor(data.color),
        },
        events = {},
    }
    local registeredMarker = {
        element = element,
        marker = data,
        pos = function(icon)
            return icon.marker.worldPos
        end,
        ---@param posData ViewportData
        onDraw = function(icon, posData)
            if not posData.viewportPos.onScreen or icon.marker.hidden then
                if icon.element.layout.props.visible then
                    icon.element.layout.props.visible = false
                    icon.element:update()
                end
                return
            end

            icon.element.layout.props.visible = true
            icon.element.layout.props.position = posData.viewportPos.pos

            icon.element.layout.props.size = baseSize * iutil.distanceScale(posData)

            icon.element:update()
        end,
        onHide = function(icon)
            icon.element.layout.props.visible = false
            icon.element:update()
        end
    }

    local focusGain = function()
        --print("focusGain: " .. aux_util.deepToString(icon.entity, 3))
        if registeredMarker.marker.note then
            local hover = {
                template = interfaces.MWUI.templates.textHeader,
                type = ui.TYPE.Text,
                alignment = ui.ALIGNMENT.End,
                props = {
                    textAlignV = ui.ALIGNMENT.Center,
                    relativePosition = util.vector2(0, 0.5),
                    text = registeredMarker.marker.note,
                    textColor = resolveColor(registeredMarker.marker.color),
                }
            }
            interfaces.LivelyMapDraw.setHoverBoxContent(hover)
        end
    end

    element.layout.events.focusGain = async:callback(focusGain)
    element.layout.events.focusLoss = async:callback(function()
        interfaces.LivelyMapDraw.setHoverBoxContent()
        return nil
    end)

    -- persist
    markerData:set(data.id, data)
    -- register in map
    markerIcons[data.id] = registeredMarker
    interfaces.LivelyMapDraw.registerIcon(registeredMarker)
end

---@param data MarkerData
local function upsertMarkerIcon(data)
    print("updateMarkerIcon: " .. aux_util.deepToString(data, 3))
    if not validateMarker(data) then
        error("upsertMarkerIcon data invalid: " .. aux_util.deepToString(data, 3))
    end
    if not markerIcons[data.id] then
        registerMarkerIcon(data)
        return
    end
    markerIcons[data.id].marker = data
    --- update UI element, too
    markerIcons[data.id].element.layout.props.color = resolveColor(data.color)
    markerIcons[data.id].element.layout.props.resource = ui.texture {
        path = resolveStampFullPath(data.iconName)
    }
    markerIcons[data.id].element:update()
    -- persist
    markerData:set(data.id, data)
end



---@param id string
---@return MarkerData?
local function getMarkerByID(id)
    if not id or (type(id) ~= "string") then
        error("getMarkerByID(): id is bad")
        return
    end
    local data = markerData:get(id)
    if not data then
        return nil
    end
    local stampData = stampPathData.fullPathToIDMap[resolveStampFullPath(data.iconName)]
    return {
        id = data.id,
        hidden = data.hidden,
        worldPos = data.worldPos,
        iconName = stampData.basename,
        iconIdx = stampData.idx,
        note = data.note,
        color = data.color,
    }
end

---@return {[string]: MarkerData}
local function getAllMarkers()
    return markerData:asTable()
end


local function onLoad()
    print("Registering saved markers...")
    for _, marker in pairs(getAllMarkers()) do
        registerMarkerIcon(marker)
    end
end


---- UI STUFF ----

local stampMakerWindow





--- must be odd and >= 3
local numColumns = 5
local previewIconSize = util.vector2(64, 64)
local windowWidth = (previewIconSize.x) * numColumns

---@class EditingMarkerData : MarkerData
---@field iconIdx number Matches the icon path.

---@type EditingMarkerData
local editingMapData = {
    color = 1,
    iconName = stampIndexToBaseName(1),
    iconIdx = 1,
    worldPos = pself.position,
    id = "placeholder",
    hidden = false,
    note = "",
}

local spacer = {
    type = ui.TYPE.Widget,
    external = {
        grow = 1
    }
}

local gridElement = ui.create {
    type = ui.TYPE.Widget,
    props = {
        --relativeSize = util.vector2(1, 1),
        size = util.vector2(windowWidth, (previewIconSize.y) * 3),
        autoSize = false,
    },
    content = ui.content {},
}
local updateGridLayout
local function setActive(idx, color)
    editingMapData.color = color
    local fullPath = resolveStampFullPath(idx)
    local iconInfo = stampPathData.fullPathToIDMap[fullPath]
    editingMapData.iconName = iconInfo.basename
    editingMapData.iconIdx = iconInfo.idx
    --print("setting active: " .. aux_util.deepToString(iconInfo, 3) .. "(fullPath is " .. fullPath .. ")")
    gridElement.layout.content = ui.content { updateGridLayout() }
    gridElement:update()
end

---@param idx number
---@param color number
local function stampPreviewLayout(idx, color)
    idx = ((idx - 1) % #stampPathData.orderedFullPaths) + 1
    color = ((color - 1) % 5) + 1

    local widget = {
        type = ui.TYPE.Widget,
        props = {
            size = previewIconSize,
        },
        events = {
            mouseClick = async:callback(function()
                print("clicked icon with idx: " ..
                    tostring(idx) ..
                    ", color: " ..
                    tostring(color) ..
                    "(current idx:" ..
                    tostring(editingMapData.iconIdx) .. ", current color: " .. tostring(editingMapData.color) .. ")")
                if idx == editingMapData.iconIdx then
                    --- cycle to next color
                    print("cycling color")
                    setActive(idx, ((editingMapData.color) % 5) + 1)
                else
                    print("changing idx")
                    setActive(idx, color)
                end
            end)
        },
        content = ui.content { {
            name = "icon",
            type = ui.TYPE.Image,
            props = {
                visible = true,
                anchor = util.vector2(0.5, 0.5),
                size = previewIconSize * 0.8,
                relativePosition = util.vector2(0.5, 0.5),
                resource = ui.texture {
                    path = resolveStampFullPath(idx)
                },
                color = resolveColor(color),
            }
        } }
    }


    widget.events.focusGain = async:callback(function()
        print("hovered icon with idx: " .. tostring(idx) .. ", color: " .. tostring(color))
        widget.content["icon"].props.color = mutil.lerpColor(resolveColor(color), util.color.rgb(1, 1, 1), 0.3)
        gridElement:update()
    end)

    widget.events.focusLoss = async:callback(function()
        widget.content["icon"].props.color = resolveColor(color)
        gridElement:update()
    end)
    return widget
end




updateGridLayout = function()
    local wingSize = math.floor(numColumns / 2)
    local makeRow = function(offset)
        local out = {}
        table.insert(out, spacer)
        for i = -wingSize, wingSize, 1 do
            local thisIdx = i + offset + editingMapData.iconIdx
            local preview = stampPreviewLayout(thisIdx, editingMapData.color)
            if thisIdx == editingMapData.iconIdx then
                table.insert(out, {
                    name = 'activeBox',
                    type = ui.TYPE.Container,
                    template = interfaces.MWUI.templates.boxSolid,
                    content = ui.content { preview }
                })
            else
                table.insert(out, preview)
            end
            table.insert(out, spacer)
        end
        return out
    end

    local main = {
        name = 'mainV',
        type = ui.TYPE.Flex,
        props = {
            horizontal = false,
            --size = util.vector2(400, 300),
            --autoSize = false,
        },
        external = {
            grow = 1
        },
        content = ui.content {
            {
                name = 'topH',
                type = ui.TYPE.Flex,
                props = {
                    horizontal = true,
                    autoSize = true,
                },
                content = ui.content {
                    unpack(makeRow(-numColumns))
                },
                external = {
                    grow = 1
                }
            },
            {
                name = 'midH',
                type = ui.TYPE.Flex,
                props = {
                    horizontal = true,
                    autoSize = true,
                },
                content = ui.content {
                    unpack(makeRow(0))
                },
                external = {
                    grow = 1
                }
            },
            {
                name = 'botH',
                type = ui.TYPE.Flex,
                props = {
                    horizontal = true,
                    autoSize = true,
                },
                content = ui.content {
                    unpack(makeRow(numColumns))
                },
                external = {
                    grow = 1
                }
            }
        }
    }
    return main
end

local defaultNote = localization("markerNote", {})
local noteBox = ui.create {
    name = "noteBox",
    type = ui.TYPE.TextEdit,
    template = interfaces.MWUI.templates.textEditLine,
    events = {
        textChanged = async:callback(function(text)
            if text == defaultNote then
                editingMapData.note = ""
            else
                editingMapData.note = text
            end
        end)
    },
    props = {
        relativePosition = util.vector2(0.5, 0.5),
        anchor = util.vector2(0.5, 0.5),
        size = util.vector2(windowWidth, 50),
        textSize = 20,
        textAlignH = ui.ALIGNMENT.Center,
        textAlignV = ui.ALIGNMENT.Center,
        text = (editingMapData.note ~= "" and editingMapData.note) or defaultNote,
        textColor = myui.interactiveTextColors.normal.default,
    }
}
--- Remove default on hover.
noteBox.layout.events.focusGain = async:callback(function()
    if noteBox.layout.props.text == defaultNote then
        noteBox.layout.props.text = ""
        noteBox:update()
    end
end)

local function resetNoteBox()
    noteBox.layout.props.text = (editingMapData.note ~= "" and editingMapData.note) or defaultNote
    noteBox:update()
end

local buttonSize = util.vector2(60, 20)

local cancelButtonElement = ui.create {}
local function updateCancelButtonElement()
    local cancelFn = function()
        print("cancel clicked")
        interfaces.LivelyMapMarker.editMarkerWindow(nil)
    end
    cancelButtonElement.layout = myui.createTextButton(
        cancelButtonElement,
        localization("cancelButton", {}),
        "normal",
        "cancelButton",
        {},
        buttonSize,
        cancelFn)
    cancelButtonElement:update()
end
updateCancelButtonElement()

local saveButtonElement = ui.create {}
local function updateSaveButtonElement()
    local saveFn = function()
        print("save clicked.")
        upsertMarkerIcon({
            color = editingMapData.color,
            hidden = false,
            iconName = editingMapData.iconName,
            id = editingMapData.id,
            note = editingMapData.note,
            worldPos = editingMapData.worldPos,
        })
        interfaces.LivelyMapMarker.editMarkerWindow(nil)
    end
    saveButtonElement.layout = myui.createTextButton(
        saveButtonElement,
        localization("saveButton", {}),
        "normal",
        "saveButton",
        {},
        buttonSize,
        saveFn)
    saveButtonElement:update()
end
updateSaveButtonElement()

local deleteButtonElement = ui.create {}
local function updateDeleteButtonElement()
    local deleteFn = function()
        print("delete clicked")
        if getMarkerByID(editingMapData.id) then
            upsertMarkerIcon({
                color = editingMapData.color,
                hidden = true, -- This does the delete
                iconName = editingMapData.iconName,
                id = editingMapData.id,
                note = editingMapData.note,
                worldPos = editingMapData.worldPos,
            })
        end
        interfaces.LivelyMapMarker.editMarkerWindow(nil)
    end
    deleteButtonElement.layout = myui.createTextButton(
        deleteButtonElement,
        localization("deleteButton", {}),
        (getMarkerByID(editingMapData.id) and "normal") or "disabled",
        "deleteButton",
        {},
        buttonSize,
        deleteFn)
    deleteButtonElement:update()
end
updateDeleteButtonElement()

stampMakerWindow = ui.create {
    name = "stampMaker",
    layer = 'Windows',
    type = ui.TYPE.Container,
    template = interfaces.MWUI.templates.boxTransparentThick,
    props = {
        relativePosition = util.vector2(0.5, 0.5),
        anchor = util.vector2(0.5, 0.5),
        visible = false,
        autoSize = true,
    },
    content = ui.content { {
        name = 'mainV',
        type = ui.TYPE.Flex,
        props = {
            relativePosition = util.vector2(0.5, 0),
            --anchor = util.vector2(0.5, 0.5),
            horizontal = false,
            --size = util.vector2(400, 400),
            --autoSize = false,
            --autoSize = true
        },
        content = ui.content {
            myui.padWidget(0, 4),
            noteBox,
            spacer,
            myui.padWidget(0, 4),
            gridElement,
            spacer,
            myui.padWidget(0, 8),
            {
                name = 'buttons',
                type = ui.TYPE.Flex,
                props = {
                    relativePosition = util.vector2(0.5, 0),
                    size = util.vector2(windowWidth, 40),
                    --anchor = util.vector2(0.5, 0.5),
                    horizontal = true,
                    --size = util.vector2(400, 400),
                    --autoSize = false,
                    --autoSize = true
                },
                content = ui.content {
                    spacer,
                    deleteButtonElement,
                    spacer,
                    cancelButtonElement,
                    spacer,
                    saveButtonElement,
                    spacer,
                },
                external = {
                    grow = 1,
                }
            },
        }
    } }
}

---@param data EditingMarkerData
local function editMarkerWindow(data)
    if not data then
        stampMakerWindow.layout.props.visible = false
        stampMakerWindow:update()
        return
    end

    if data.id then
        local found = getMarkerByID(data.id)
        if found then
            print("found existing marker: " .. aux_util.deepToString(found, 3))
            data = found
        end
    end

    editingMapData = data

    -- Keep paths valid.
    local stampFullPath = resolveStampFullPath(editingMapData.iconName)
    if not stampFullPath then
        stampFullPath = resolveStampFullPath(editingMapData.iconIdx)
    end
    if not stampFullPath then
        stampFullPath = resolveStampFullPath(1)
        --- TODO: this is busted
        print("DEFAULT STAMP!!!!! " .. aux_util.deepToString(editingMapData, 3))
    end
    local stampInfo = stampPathData.fullPathToIDMap[stampFullPath]
    editingMapData.iconName = stampInfo.basename
    editingMapData.iconIdx = stampInfo.idx

    -- Keep color valid.
    if not editingMapData.color then
        editingMapData.color = math.random(5)
    end

    -- Default position/note.
    if not editingMapData.worldPos then
        editingMapData.worldPos = pself.position
        if not editingMapData.note then
            editingMapData.note = pself.cell.name
        end
    end

    if not editingMapData.id then
        error("editMarkerWindow missing id")
    end

    -- Note shouldn't be nil.
    if not editingMapData.note then
        editingMapData.note = ""
    end

    if not validateMarker(editingMapData) then
        error("failed to build map data: " .. aux_util.deepToString(editingMapData, 3))
    end

    setActive(editingMapData.iconIdx, editingMapData.color or 1)
    resetNoteBox()
    updateDeleteButtonElement()
    updateCancelButtonElement()
    updateSaveButtonElement()
    stampMakerWindow.layout.props.visible = true
    stampMakerWindow:update()
end

--- debugging
editMarkerWindow({ id = "334324" })


return {
    interfaceName = MOD_NAME .. "Marker",
    interface = {
        version = 1,
        getMarkerByID = getMarkerByID,
        upsertMarkerIcon = upsertMarkerIcon,
        editMarkerWindow = editMarkerWindow,
    },
    engineHandlers = {
        onLoad = onLoad,
    },
}
