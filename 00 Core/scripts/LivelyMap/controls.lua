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

local currentMapData = nil

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

local function facing2D(camViewVector)
    local viewDir = camViewVector or camera.viewportToWorldVector(util.vector2(0.5, 0.5))
    return util.vector3(viewDir.x, viewDir.y, 0):normalize()
end

-- moveCamera safely moves the camera within acceptable bounds.
local function moveCamera(data)
    if data == nil then
        return
    end

    if data.pitch then
        camera.setPitch(data.pitch)
    end
    if data.yaw then
        camera.setYaw(data.yaw)
    end
    if data.position or data.relativePosition then
        local pos = data.position or (camera.getPosition() + data.relativePosition)
        camera.setStaticPosition(pos)
    end
end

-- Lerp the camera to a new position.
local trackInfo = {
    tracking = false,
    startTime = 0,
    endTime = 0,
    startPos = nil,
    endPos = nil,
    -- onEnd is invoked when tracking stops.
    -- the parameter will be true if it completed normally
    -- (it will be false if interrupted)
    onEnd = nil
}

local function advanceTracker()
    if not trackInfo.tracking then
        return
    end
    local currentTime = core.getRealTime()
    local intermediatePosition = nil
    local i = 0
    if currentTime >= trackInfo.endTime then
        -- set to end
        i = 1
        intermediatePosition = trackInfo.endPos
    else
        -- lerp!
        i = util.remap(currentTime, trackInfo.startTime, trackInfo.endTime, 0, 1)
        intermediatePosition = mutil.lerpVec3(i, trackInfo.startPos, trackInfo.endPos)
    end

    if i >= 1 then
        trackInfo.tracking = false
        if trackInfo.onEnd then
            trackInfo.onEnd(true)
        end
    end

    -- TODO: maybe do fancy camera movements in the future
    moveCamera({ position = intermediatePosition })
end

local function trackPosition(newCameraPos, duration, onEnd)
    if newCameraPos == nil then
        error("trackPosition newCameraPos is required.")
    end
    if duration and type(duration) ~= "number" then
        error("trackPosition duration should be a number, not a " .. type(duration))
    end
    if onEnd and type(onEnd) ~= "function" then
        error("trackPosition onEnd should be a function, not a " .. type(onEnd))
    end
    trackInfo.tracking = true
    trackInfo.startPos = camera.getPosition()
    trackInfo.endPos = newCameraPos
    trackInfo.startTime = core.getRealTime()
    duration = duration or 0
    trackInfo.endTime = trackInfo.startTime + duration
    trackInfo.onEnd = onEnd

    -- Immediate advance if no duration given.
    if duration <= 0 then
        advanceTracker()
    end
end

local vecForward = util.vector3(0, 1, 0)
local vecBackward = vecForward * -1
local vecRight = util.vector3(1, 0, 0)
local vecLeft = vecRight * -1
local moveSpeed = 100

local function onFrame(dt)
    -- Fake a duration if we're paused.
    if dt <= 0 then
        dt = 1 / 60
    end
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
    local hasInput = keys.forward.pressed or keys.backward.pressed or keys.left.pressed or keys.right.pressed
    if hasInput then
        local moveVec = (vecForward * keys.forward.analog +
            vecBackward * keys.backward.analog +
            vecRight * keys.right.analog +
            vecLeft * keys.left.analog):normalize() * moveSpeed * dt

        moveCamera({
            relativePosition = moveVec
        })
        -- Interrupt tracking
        if trackInfo.tracking then
            if trackInfo.onEnd then
                trackInfo.onEnd(false)
            end
        end
        trackInfo.tracking = false
    else
        advanceTracker()
    end
end

-- cameraOffset returns a vector offset for the camera position
-- so that the center of the viewPort lands on targetPosition.
local function cameraOffset(targetPosition, camPitch, camViewVector)
    local pos = targetPosition or camera.getPosition()
    local pitch = camPitch or camera.getPitch()
    local height = pos.z - currentMapData.object.position.z
    local viewDir = facing2D(camViewVector)
    print(viewDir)
    -- 1.5708 - pitch is the angle between straight down and camera center.
    return viewDir * (-1 * height * math.tan(1.5708 - pitch))
end

local defaultHeight = 200
local defaultPitch = 1
local function onMapMoved(data)
    print("controls.onMapMoved")
    currentMapData = data
    -- If this is not a swap, then this is a brand new map session.
    if not data.swapped then
        -- Orient the camera so starting position is in the center.
        startCamera()
        camera.setYaw(0)
        local mapCenter = data.object:getBoundingBox().center
        local cellPos = mutil.worldPosToCellPos(data.startWorldPosition)
        local rel = putil.relativeCellPos(currentMapData, cellPos)
        local mapWorldPos = putil.relativeCellPosToMapPos(currentMapData, rel)
        local heightOffset = util.vector3(0, 0, defaultHeight)
        local camOffset = cameraOffset(mapCenter + heightOffset, defaultPitch, util.vector3(0, 1, 0))

        moveCamera({
            pitch = defaultPitch,
            position = mapWorldPos + camOffset + heightOffset,
        })
    end
end

local function onMapHidden(data)
    print("controls.onMapHidden")
    currentMapData = nil
    -- If it's not a swap, it means we are done looking at the map.
    if not data.swapped then
        endCamera()
    end
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
