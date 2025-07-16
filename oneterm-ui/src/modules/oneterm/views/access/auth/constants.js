export const TARGET_SELECT_TYPE = {
  ALL: 'all',
  ID: 'ids',
  REGEX: 'regex',
  TAG: 'tags'
}

export const TARGET_SELECT_TYPE_NAME = {
  [TARGET_SELECT_TYPE.ALL]: 'oneterm.auth.all',
  [TARGET_SELECT_TYPE.ID]: 'oneterm.auth.selectItem',
  [TARGET_SELECT_TYPE.REGEX]: 'oneterm.auth.regex',
  [TARGET_SELECT_TYPE.TAG]: 'oneterm.auth.tags',
}
