--[[
ErnPerkFramework for OpenMW.
Copyright (C) 2025 Erin Pentecost

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
local interfaces   = require("openmw.interfaces")
local storage      = require("openmw.storage")
local MOD_NAME     = require("scripts.LivelyMap.ns")
local util         = require('openmw.util')
local input        = require('openmw.input')

local psoGroupKey  = "Settings/" .. MOD_NAME .. "/pso"
local mainGroupKey = "Settings/" .. MOD_NAME

local function init()
    interfaces.Settings.registerPage {
        key = MOD_NAME,
        l10n = MOD_NAME,
        name = "name",
    }

    input.registerAction {
        key = MOD_NAME .. "_ToggleMapWindow",
        type = input.ACTION_TYPE.Boolean,
        l10n = MOD_NAME,
        defaultValue = false,
    }

    interfaces.Settings.registerGroup {
        key = psoGroupKey,
        page = MOD_NAME,
        l10n = MOD_NAME,
        name = "psoName",
        description = "psoDescription",
        permanentStorage = true,
        settings = {
            {
                key = "psoUnlock",
                name = "psoUnlockName",
                description = "psoUnlockDescription",
                default = false,
                renderer = "checkbox",
            },
            {
                key = "psoDepth",
                name = "psoDepthName",
                description = "psoDepthDescription",
                default = 0,
                renderer = "number",
                argument = {
                    integer = true,
                    min = 0,
                    max = 300,
                }
            },
            {
                key = "psoPushdownOnly",
                name = "psoPushdownOnlyName",
                description = "psoPushdownOnlyDescription",
                default = true,
                renderer = "checkbox",
            },
        }
    }

    interfaces.Settings.registerGroup {
        key = mainGroupKey,
        page = MOD_NAME,
        l10n = MOD_NAME,
        name = "settings",
        permanentStorage = true,
        settings = {
            {
                key = "extendDetectRange",
                name = "extendDetectRangeName",
                description = "extendDetectRangeDescription",
                default = true,
                renderer = "checkbox",
            },
            {
                key = "drawLimitNeravarinesJourney",
                name = "drawLimitNeravarinesJourneyName",
                description = "drawLimitNeravarinesJourneyDescription",
                default = true,
                renderer = "checkbox",
            },
            {
                key = "palleteColor1",
                name = "palleteColor1Name",
                renderer = MOD_NAME .. "color",
                default = util.color.hex("FFBE0B"),
            },
            {
                key = "palleteColor2",
                name = "palleteColor2Name",
                renderer = MOD_NAME .. "color",
                default = util.color.hex("FB5607"),
            },
            {
                key = "palleteColor3",
                name = "palleteColor3Name",
                renderer = MOD_NAME .. "color",
                default = util.color.hex("FF006E"),
            },
            {
                key = "palleteColor4",
                name = "palleteColor4Name",
                renderer = MOD_NAME .. "color",
                default = util.color.hex("8338EC"),
            },
            {
                key = "palleteColor5",
                name = "palleteColor5Name",
                renderer = MOD_NAME .. "color",
                default = util.color.hex("3A86FF"),
            },
            {
                key = "debug",
                name = "debugName",
                default = false,
                renderer = "checkbox",
            },
        }
    }
end

local lookupFuncTable = {
    __index = function(table, key)
        if key == "subscribe" then
            return function(callback)
                return table.section.subscribe(table.section, callback)
            end
        elseif key == "section" then
            return table.section
        elseif key == "groupKey" then
            return table.groupKey
        end
        -- fall through to settings section
        local val = table.section:get(key)
        if val ~= nil then
            return val
        else
            error("unknown setting " .. tostring(key))
        end
    end,
}

local mainContainer = {
    groupKey = mainGroupKey,
    section = storage.playerSection(mainGroupKey)
}
setmetatable(mainContainer, lookupFuncTable)

local psoContainer = {
    groupKey = psoGroupKey,
    section = storage.playerSection(psoGroupKey)
}
setmetatable(psoContainer, lookupFuncTable)

return {
    init = init,
    main = mainContainer,
    pso = psoContainer,
}
