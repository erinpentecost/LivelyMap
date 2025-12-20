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
local input           = require('openmw.input')
local heightData      = storage.globalSection(MOD_NAME .. "_heightData")
local keytrack        = require("scripts.ErnOneStick.keytrack")
local uiInterface     = require("openmw.interfaces").UI

local controls        = require('openmw.interfaces').Controls
local cameraInterface = require("openmw.interfaces").Camera

-- Exit the map when one of these triggers goes off:
for _, exitTrigger in ipairs { "Journal", "Inventory", "GameMenu" } do
    input.registerTriggerHandler(exitTrigger, async:callback(function()
        core.sendGlobalEvent(MOD_NAME .. "onHideMap", { player = pself })
    end))
end

-- Track inputs we need for navigating the map.
local keys = {
    forward  = keytrack.NewKey("forward", function(dt)
        return input.getRangeActionValue("MoveForward")
    end),
    backward = keytrack.NewKey("backward", function(dt)
        return input.getRangeActionValue("MoveBackward")
    end),
    left     = keytrack.NewKey("left", function(dt)
        return input.getRangeActionValue("MoveLeft")
    end),
    right    = keytrack.NewKey("right", function(dt)
        return input.getRangeActionValue("MoveRight")
    end)
}

-- Reset inputs so they don't get stuck.
local function clearControls()
    pself.controls.sideMovement = 0
    pself.controls.movement = 0
    pself.controls.yawChange = 0
    pself.controls.pitchChange = 0
    pself.controls.run = false
end

-- Store old camera state so we can reset to that same state
-- once we exit the map.
local cameraState = nil
local function startCamera()
    controls.overrideMovementControls(true)
    cameraInterface.disableModeControl(MOD_NAME)
    controls.overrideUiControls(true)
    uiInterface.setHudVisibility(false)
    clearControls()
    if cameraState == nil then
        -- Don't override the old state.
        -- this might be called multiple times before
        -- endCamera() is called.
        cameraState = {
            pitch = camera.getPitch(),
            yaw = camera.getYaw(),
            roll = camera.getRoll(),
            position = camera.getPosition(),
            mode = camera.getMode()
        }
    end
    camera.setMode(camera.MODE.Static, true)
end

local function endCamera()
    controls.overrideMovementControls(false)
    cameraInterface.enableModeControl(MOD_NAME)
    controls.overrideUiControls(false)
    uiInterface.setHudVisibility(true)
    clearControls()
    if cameraState then
        camera.setMode(cameraState.mode, true)
        camera.setPitch(cameraState.pitch)
        camera.setYaw(cameraState.yaw)
        camera.setRoll(cameraState.roll)
    end
    cameraState = nil
end

-- Pan the camera to a specific point of interest.
local trackInfo = {
    startTime = 0,
    endTime = 0,
    startPos = nil,
    endPos = nil,
}
local function trackPosition(worldPos, time)
    -- TODO: don't use camera position for startPos.
    -- instead, get the current pos on the map at center of screen.
    trackInfo.startPos = camera.getPosition()
    trackInfo.endPos = worldPos
    trackInfo.startTime = core.getRealTime()
    trackInfo.endTime = trackInfo.startTime + time
end

local function onFrame(dt)
    -- Only track inputs while the map is up.
    if not cameraState then
        return
    end
    -- Track inputs.
    keys.forward:update(dt)
    keys.backward:update(dt)
    keys.left:update(dt)
    keys.right:update(dt)
    -- If we have input, cancel trackPosition,
    -- then move the camera manually.
    -- Else, advance the camera toward tracked position.
end

local function onMapMoved(data)
    print("controls.onMapMoved")
    startCamera()
    -- Move camera into starting position
    local cellPos = mutil.worldPosToCellPos(data.startWorldPosition)
    local rel = putil.relativeCellPos(data, cellPos)
    local mapWorldPos = putil.relativeCellPosToMapPos(data, rel)

    --local mapObj = data.object.position
    camera.setStaticPosition(mapWorldPos + util.vector3(0, 0, 200))
    camera.setPitch(1)
    camera.setYaw(0)
end

local function onMapHidden(data)
    print("controls.onMapHidden")
    endCamera()
end

return {
    interfaceName = MOD_NAME .. "Controls",
    interface = {
        version = 1,
        trackPosition = trackPosition,
    },
    engineHandlers = {
        onFrame = onFrame,
    },
    eventHandlers = {
        [MOD_NAME .. "onMapMoved"] = onMapMoved,
        [MOD_NAME .. "onMapHidden"] = onMapHidden,
    },
}
