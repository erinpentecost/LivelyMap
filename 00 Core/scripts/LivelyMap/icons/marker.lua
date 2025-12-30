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

---comment
---@param data MarkerData
local function registerMarker(data)
    local element = ui.create {
        type = ui.TYPE.Image,
        props = {
            visible = true,
            position = util.vector2(100, 100),
            anchor = util.vector2(0.5, 0.5),
            size = baseSize,
            resource = ui.texture {
                path = data.iconPath
            },
            color = resolveColor(data.color),
        },
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

    -- register in map
    markerIcons[data.id] = registeredMarker
    interfaces.LivelyMapDraw.registerIcon(registeredMarker)
end

---@param data MarkerData
local function updateMarker(data)
    if markerIcons[data.id] == nil then
        error("updating marker that doesn't exist")
        return
    end
    markerIcons[data.id].marker = data
    --- update UI element, too
    markerIcons[data.id].element.layout.props.color = resolveColor(data.color)
    markerIcons[data.id].element.layout.props.resource = ui.texture {
        path = data.iconPath
    }
    markerIcons[data.id].element:update()
end

---@class MarkerData
---@field id string Unique internal ID.
---@field hidden boolean Basically soft-delete.
---@field worldPos util.vector3
---@field iconPath string Full path to the image file.
---@field note string? This appears in the hover box.
---@field color number This corresponds to the pallete color number.

---@param id string
---@return MarkerData?
local function getMarkerByID(id)
    if not id or (type(id) ~= "string") then
        error("getMarkerByID(): id is bad")
        return
    end
    return markerData:get(id)
end

---@return {[string]: MarkerData}
local function getAllMarkers()
    return markerData:asTable()
end

---Makes a new marker if it does not exist.
---@param data MarkerData
local function newMarker(data)
    if not data or not data.id or not type(data.id) == "string" then
        error("newMarker: bad data")
    end
    if not getMarkerByID(data.id) then
        markerData:set(data.id, data)
        registerMarker(data)
    end
end

---@param data MarkerData
local function upsertMarker(data)
    if not data or not data.id or not type(data.id) == "string" then
        error("newMarker: bad data")
    end

    local exists = getMarkerByID(data.id) ~= nil
    markerData:set(data.id, data)
    if exists then
        updateMarker(data)
    else
        registerMarker(data)
    end
end


local function onLoad()
    print("Registering saved markers...")
    for _, marker in pairs(getAllMarkers()) do
        registerMarker(marker)
    end
end


--- UI stuff
--- it'll be a 3x5 grid.
--- left/right will scan through icons
--- up/down will scan through colors
--- textbox below for optional note. defaults to name of cell.
--- then ok/cancel buttons
---

--- stable list of all available stamps
local function stampList()
    local out = {}
    for stampPath in vfs.pathsWithPrefix("textures\\LivelyMap\\stamps") do
        table.insert(out, stampPath)
    end
    return out
end
local stampPaths = stampList()


--- must be odd and >= 3
local numColumns = 7
local previewIconSize = util.vector2(64, 64)

local activeColor = 1
local activeIdx = 1

local gridElement = ui.create {
    type = ui.TYPE.Widget,
    props = {
        --relativeSize = util.vector2(1, 1),
        size = util.vector2((previewIconSize.x) * numColumns, 300),
        autoSize = false,
    },
    content = ui.content {},
}
local updateGridLayout
local function setActive(idx, color)
    activeIdx = idx
    activeColor = color
    gridElement.layout.content = ui.content { updateGridLayout(idx, color) }
    gridElement:update()
end



---comment
---@param idx number
---@param color number
---@param sizeFactor number?
local function stampPreviewLayout(idx, color, sizeFactor)
    idx = ((idx - 1) % #stampPaths) + 1
    color = ((color - 1) % 5) + 1
    if sizeFactor == nil then
        sizeFactor = 1
    end

    local widget = {
        type = ui.TYPE.Widget,
        props = {
            size = previewIconSize,
        },
        events = {
            mouseClick = async:callback(function()
                print("click!")
                setActive(idx, color)
            end)
        },
        content = ui.content { {
            name = "icon",
            type = ui.TYPE.Image,
            props = {
                visible = true,
                anchor = util.vector2(0.5, 0.5),
                size = previewIconSize * sizeFactor,
                relativePosition = util.vector2(0.5, 0.5),
                resource = ui.texture {
                    path = stampPaths[idx]
                },
                color = resolveColor(color),
            }
        } }
    }


    widget.events.focusGain = async:callback(function()
        widget.content["icon"].props.color = mutil.lerpColor(resolveColor(color), util.color.rgb(1, 1, 1), 0.3)
        gridElement:update()
    end)

    widget.events.focusLoss = async:callback(function()
        widget.content["icon"].props.color = resolveColor(color)
        gridElement:update()
    end)
    return widget
end




updateGridLayout = function(idx, color)
    local spacer = {
        type = ui.TYPE.Widget,
        external = {
            grow = 1
        }
    }

    local makeRow = function(rowColor, altSize)
        local out = {}
        local wingSize = math.floor(numColumns / 2)
        table.insert(out, spacer)
        for i = -wingSize, wingSize, 1 do
            local preview = stampPreviewLayout(activeIdx + i, rowColor, altSize)
            if i == 0 and rowColor == activeColor then
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

    local altSize = 0.5
    local main = {
        name = 'mainV',
        type = ui.TYPE.Flex,
        props = {
            horizontal = false,
            --size = util.vector2(400, 300),
            --autoSize = false,
        },
        content = ui.content {
            {
                name = 'topH',
                type = ui.TYPE.Flex,
                props = {
                    horizontal = true,
                    --size = util.vector2(400, 100),
                    --autoSize = false,
                    --relativeSize = util.vector2(1, 0.3),
                    autoSize = true,
                },
                content = ui.content {
                    unpack(makeRow(color - 1, altSize))
                },
                external = {
                    grow = altSize
                }
            },
            {
                name = 'midH',
                type = ui.TYPE.Flex,
                props = {
                    horizontal = true,
                    --size = util.vector2(400, 100),
                    --autoSize = false,
                    --relativeSize = util.vector2(1, 0.3 * altSize),
                    autoSize = true,
                },
                content = ui.content {
                    unpack(makeRow(color, 0.9))
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
                    --size = util.vector2(400, 100),
                    --autoSize = false,
                    --relativeSize = util.vector2(1, 0.3 * altSize),
                    autoSize = true,
                },
                content = ui.content {
                    unpack(makeRow(color + 1, altSize))
                },
                external = {
                    grow = altSize
                }
            }
        }
    }
    return main
end

local stampMakerWindow = ui.create {
    name = "stampMaker",
    layer = 'Windows',
    type = ui.TYPE.Container,
    template = interfaces.MWUI.templates.boxTransparentThick,
    props = {
        relativePosition = util.vector2(0.5, 0.5),
        anchor = util.vector2(0.5, 0.5),
        visible = true,
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
            autoSize = true
        },
        content = ui.content {
            gridElement
        }
    } }
}

setActive(1, 1)

return {
    interfaceName = MOD_NAME .. "Marker",
    interface = {
        version = 1,
        getMarkerByID = getMarkerByID,
        newMarker = newMarker,
        upsertMarker = upsertMarker,
    },
    engineHandlers = {
        onLoad = onLoad,
    },
}
