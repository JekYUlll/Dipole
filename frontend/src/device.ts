const storageKey = 'dipole.web.deviceID'

export function getDeviceID(): string {
  const existing = localStorage.getItem(storageKey)
  if (existing) return existing

  const deviceID = typeof crypto.randomUUID === 'function'
    ? crypto.randomUUID()
    : `web-${Date.now()}-${Math.random().toString(16).slice(2)}`
  localStorage.setItem(storageKey, deviceID)
  return deviceID
}
