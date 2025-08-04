export const PROTOCOL_ICON = {
  'ssh': 'a-oneterm-ssh2',
  'rdp': 'a-oneterm-ssh1',
  'vnc': 'oneterm-rdp',
  'telnet': 'a-telnet1',
  'redis': 'oneterm-redis',
  'mysql': 'oneterm-mysql',
  'mongodb': 'a-mongoDB1',
  'postgresql': 'a-postgreSQL1',
  'https': 'oneterm-https',
  'http': 'oneterm-http'
}

export const protocolSelectOption = [
  {
    title: 'oneterm.assetList.basic',
    list: [
      {
        key: 'ssh',
        label: 'SSH',
        icon: PROTOCOL_ICON['ssh']
      },
      {
        key: 'rdp',
        label: 'RDP',
        icon: PROTOCOL_ICON['rdp']
      },
      {
        key: 'vnc',
        label: 'VNC',
        icon: PROTOCOL_ICON['vnc']
      },
      {
        key: 'telnet',
        label: 'Telnet',
        icon: PROTOCOL_ICON['telnet']
      },
    ]
  },
  {
    title: 'oneterm.assetList.database',
    list: [
      {
        key: 'redis',
        label: 'Redis',
        icon: PROTOCOL_ICON['redis']
      },
      {
        key: 'mysql',
        label: 'MySQL',
        icon: PROTOCOL_ICON['mysql']
      },
      {
        key: 'mongodb',
        label: 'MongoDB',
        icon: PROTOCOL_ICON['mongodb']
      },
      {
        key: 'postgresql',
        label: 'PostgreSQL',
        icon: PROTOCOL_ICON['postgresql']
      }
    ]
  },
  {
    title: 'Web',
    list: [
      {
        key: 'https',
        label: 'HTTPS',
        icon: PROTOCOL_ICON['https']
      },
      {
        key: 'http',
        label: 'HTTP',
        icon: PROTOCOL_ICON['http']
      }
    ]
  }
]

export const protocolMap = {
  'ssh': 22,
  'rdp': 3389,
  'vnc': 5900,
  'telnet': 23,
  'redis': 6379,
  'mysql': 3306,
  'mongodb': 27017,
  'postgresql': 5432,
  'https': 443,
  'http': 80
}

export const ACCESS_POLICY = {
  FULL_ACCESS: 'full_access',
  READ_ONLY: 'read_only'
}

export const ACCESS_POLICY_NAME = {
  [ACCESS_POLICY.FULL_ACCESS]: 'oneterm.web.fullAccess',
  [ACCESS_POLICY.READ_ONLY]: 'oneterm.web.readOnly',
}

export const AUTH_MODE = {
  NONE: 'none',
  SMART: 'smart',
  MANUAL: 'manual'
}

export const AUTH_MODE_NAME = {
  [AUTH_MODE.NONE]: 'oneterm.web.noAuthenticationRequired',
  [AUTH_MODE.SMART]: 'oneterm.web.autoLogin',
  [AUTH_MODE.MANUAL]: 'oneterm.web.manualLogin',
}

export const DEFAULT_WEB_CONFIG = {
  access_policy: ACCESS_POLICY.FULL_ACCESS,
  auth_mode: AUTH_MODE.NONE,
  login_accounts: [],
  proxy_settings: {
    allowed_methods: [],
    blocked_paths: [],
    max_concurrent: 1,
    recording_enabled: true,
    watermark_enabled: true
  }
}
