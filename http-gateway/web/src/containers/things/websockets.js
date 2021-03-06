import { thingsApiEndpoints } from './constants'

import { store, history } from '@/store'
import { Emitter } from '@/common/services/emitter'
import { showInfoToast } from '@/components/toast'
import {
  thingsStatuses,
  resourceEventTypes,
  THINGS_STATUS_WS_KEY,
  THINGS_REGISTERED_UNREGISTERED_COUNT_EVENT_KEY,
} from './constants'
import {
  getThingNotificationKey,
  getResourceRegistrationNotificationKey,
  getResourceUpdateNotificationKey,
} from './utils'
import { isNotificationActive } from './slice'
import { getThingApi } from './rest'
import { messages as t } from './things-i18n'

const { ONLINE, REGISTERED, UNREGISTERED } = thingsStatuses
const DEFAULT_NOTIFICATION_DELAY = 500

// WebSocket listener for device status change.
export const deviceStatusListener = async message => {
  const notificationsEnabled = isNotificationActive(THINGS_STATUS_WS_KEY)(
    store.getState()
  )

  setTimeout(async () => {
    const { deviceIds, status } = JSON.parse(message.data)
    try {
      deviceIds.forEach(async deviceId => {
        // Emit an event: things.status.{deviceId}
        Emitter.emit(`${THINGS_STATUS_WS_KEY}.${deviceId}`, {
          deviceId,
          status,
        })

        // Get the notification state of a single device from redux store
        const currentThingNotificationsEnabled = isNotificationActive(
          getThingNotificationKey(deviceId)
        )(store.getState())

        // Show toast
        if (
          (notificationsEnabled || currentThingNotificationsEnabled) &&
          status !== UNREGISTERED
        ) {
          const {
            data: { device: { n: deviceName } = {} } = {},
          } = await getThingApi(deviceId)
          const toastMessage =
            status === ONLINE ? t.thingWentOnline : t.thingWentOffline
          showInfoToast(
            {
              title: t.thingStatusChange,
              message: { message: toastMessage, params: { name: deviceName } },
            },
            {
              onClick: () => {
                history.push(`/things/${deviceId}`)
              },
              isNotification: true,
            }
          )
        }
      })
    } catch (error) {} // ignore error

    // If the event was registered or unregistered, emit an event with the number to increment by
    if ([REGISTERED, UNREGISTERED].includes(status)) {
      // Emit an event: things-registered-unregistered-count
      Emitter.emit(
        THINGS_REGISTERED_UNREGISTERED_COUNT_EVENT_KEY,
        deviceIds.length
      )
    }
  }, notificationsEnabled ? DEFAULT_NOTIFICATION_DELAY : 0)
}

export const deviceResourceRegistrationListener = ({
  deviceId,
  deviceName,
}) => message => {
  // Things notifications must be enabled to see a toast message
  const notificationsEnabled = isNotificationActive(
    getThingNotificationKey(deviceId)
  )(store.getState())

  const parsedMessageData = JSON.parse(message.data)
  const { resources, event } = parsedMessageData
  const resourceRegistrationObservationWSKey = getResourceRegistrationNotificationKey(
    deviceId
  )

  resources.forEach(resource => {
    const { href } = resource

    // Emit an event: things.resource.registration.{deviceId}.{href}.{event}
    Emitter.emit(`${resourceRegistrationObservationWSKey}.${href}.${event}`, {
      event,
      resource,
    })

    if (notificationsEnabled) {
      const isNew = event === resourceEventTypes.ADDED
      const toastTitle = isNew ? t.newResource : t.resourceDeleted
      const toastMessage = isNew
        ? t.resourceAdded
        : t.resourceWithHrefWasDeleted
      const onClickAction = () => {
        if (isNew) {
          // redirect to resource and open resource modal
          history.push(`/things/${deviceId}${href}`)
        } else {
          // redirect to device
          history.push(`/things/${deviceId}`)
        }
      }
      // Show toast
      showInfoToast(
        {
          title: toastTitle,
          message: {
            message: toastMessage,
            params: { href, deviceName, deviceId },
          },
        },
        {
          onClick: onClickAction,
          isNotification: true,
        }
      )
    }
  })
}

export const deviceResourceUpdateListener = ({
  deviceId,
  href,
  deviceName,
}) => message => {
  const eventKey = getResourceUpdateNotificationKey(deviceId, href)
  const notificationsEnabled = isNotificationActive(eventKey)(store.getState())

  const parsedMessageData = JSON.parse(message.data)

  // Emit an event: things.resource.update.{deviceId}.{href}
  Emitter.emit(`${eventKey}`, parsedMessageData)

  if (notificationsEnabled) {
    // Show toast
    showInfoToast(
      {
        title: t.resourceUpdated,
        message: {
          message: t.resourceUpdatedDesc,
          params: { href, deviceName },
        },
      },
      {
        onClick: () => {
          history.push(`/things/${deviceId}${href}`)
        },
        isNotification: true,
      }
    )
  }
}

export const thingsWSClient = {
  name: THINGS_STATUS_WS_KEY,
  api: thingsApiEndpoints.THINGS_WS,
  listener: deviceStatusListener,
}
