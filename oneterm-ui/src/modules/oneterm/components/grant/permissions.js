export const ACTION_LABELS = {
  read: 'view',
  request_retrieve: 'oneterm.passwordView.requestPermission',
  retrieve: 'oneterm.credential.permissions.retrieve',
  write: 'update',
  delete: 'delete',
  grant: 'grant',
  update_secret: 'oneterm.credential.permissions.updateSecret',
  verify_secret: 'oneterm.credential.permissions.verifySecret',
  rotate_secret: 'oneterm.credential.permissions.rotateSecret',
  recover_secret: 'oneterm.credential.permissions.recoverSecret',
  manage_policy: 'oneterm.credential.permissions.managePolicy',
  request_access: 'oneterm.credential.permissions.requestAccess'
}

export function permissionColumns(actions) {
  const unique = [...new Set(actions.filter((action) => typeof action === 'string' && action))]
  const known = Object.keys(ACTION_LABELS).filter((action) => unique.includes(action))
  return [...known, ...unique.filter((action) => !known.includes(action))]
    .map((field) => ({ field, titleKey: ACTION_LABELS[field] || '', label: field }))
}

export function emptyPermissions(columns) {
  return Object.fromEntries(columns.map(({ field }) => [field, false]))
}
