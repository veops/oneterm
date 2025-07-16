export const COMMAND_CATEGORY = {
  SECURITY: 'security',
  SYSTEM: 'system',
  DATABASE: 'database',
  NETWORK: 'network',
  FILE: 'file',
  DEVELOPER: 'developer',
  CUSTOM: 'custom'
}

export const COMMAND_CATEGORY_NAME = {
  [COMMAND_CATEGORY.SECURITY]: 'oneterm.commandFilter.securityRelated',
  [COMMAND_CATEGORY.SYSTEM]: 'oneterm.commandFilter.systemOperations',
  [COMMAND_CATEGORY.DATABASE]: 'oneterm.commandFilter.databaseOperations',
  [COMMAND_CATEGORY.NETWORK]: 'oneterm.commandFilter.networkOperations',
  [COMMAND_CATEGORY.FILE]: 'oneterm.commandFilter.fileOperations',
  [COMMAND_CATEGORY.DEVELOPER]: 'oneterm.commandFilter.developmentRelated',
  [COMMAND_CATEGORY.CUSTOM]: 'other'
}

export const COMMAND_RISK_NAME = {
  0: 'oneterm.commandFilter.safe',
  1: 'oneterm.commandFilter.warning',
  2: 'oneterm.commandFilter.dangerous',
  3: 'oneterm.commandFilter.criticalDanger'
}
