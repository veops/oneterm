// @ts-ignore
import { BasicLayout, RouteView } from '@/layouts'
import user from '@/store/global/user'

const genOnetermRoutes = () => {
    return {
        path: '/oneterm',
        name: 'oneterm',
        component: BasicLayout,
        meta: { title: 'oneterm.menu.oneterm', keepAlive: false },
        redirect: () => {
            const { detailPermissions } = user.state

            if (detailPermissions['oneterm'].some(item => item.name === 'WorkStation')) {
                return '/oneterm/workstation'
            }
            if (detailPermissions['oneterm'].some(item => item.name === 'Dashboard')) {
                return '/oneterm/dashboard'
            }
            if (detailPermissions['oneterm'].some(item => item.name === 'Public_Key')) {
                return '/oneterm/publickey'
            }

            return '/oneterm/workstation'
        },
        children: [
          {
              path: '/oneterm/dashboard',
              name: 'onterm_dashboard',
              component: () => import('../views/dashboard'),
              meta: { title: 'dashboard', appName: 'oneterm', icon: 'ops-oneterm-dashboard', selectedIcon: 'ops-oneterm-dashboard-selected', keepAlive: false, permission: ['oneterm_admin', 'Dashboard'] }
          },
          {
              path: '/oneterm/workstation',
              name: 'onterm_work_station',
              component: () => import('../views/workStation'),
              meta: {
                title: 'oneterm.menu.workStation', appName: 'oneterm', icon: 'ops-oneterm-workstation', selectedIcon: 'ops-oneterm-workstation-selected', keepAlive: false, permission: ['WorkStation', 'oneterm_admin']
              }
          },
          {
              path: '/oneterm/publickey',
              name: 'onterm_public_key',
              component: () => import('../views/publicKey'),
              meta: { title: 'oneterm.menu.publicKey', appName: 'oneterm', keepAlive: false, icon: 'ops-oneterm-publickey', selectedIcon: 'ops-oneterm-publickey-selected', permission: ['oneterm_admin', 'Public_Key'] }
          },
          {
              path: '/oneterm/assets',
              name: 'oneterm_assets',
              component: RouteView,
              meta: { title: 'oneterm.menu.assetManagement', appName: 'oneterm', icon: 'ops-oneterm-assets', selectedIcon: 'ops-oneterm-assets-selected', permission: ['oneterm_admin', 'Assets', 'Accounts', 'Gateways', 'Security'] },
              redirect: '/oneterm/assets/assets',
              children: [{
                  path: '/oneterm/assetlist',
                  name: 'oneterm_asset_list',
                  meta: { title: 'oneterm.menu.assets', appName: 'oneterm', icon: 'ops-oneterm-assetlist', selectedIcon: 'ops-oneterm-assetlist-selected', permission: ['oneterm_admin', 'Assets'] },
                  component: () => import('../views/assets/assets')
              }, {
                path: '/oneterm/account',
                name: 'oneterm_account',
                meta: { title: 'oneterm.menu.accounts', appName: 'oneterm', icon: 'ops-oneterm-account', selectedIcon: 'ops-oneterm-account-selected', permission: ['oneterm_admin', 'Accounts'] },
                component: () => import('../views/assets/account')
              }, {
                  path: '/oneterm/gateway',
                  name: 'oneterm_gateway',
                  meta: { title: 'oneterm.menu.gateways', appName: 'oneterm', icon: 'ops-oneterm-gateway', selectedIcon: 'ops-oneterm-gateway-selected', permission: ['oneterm_admin', 'Gateways'] },
                  component: () => import('../views/assets/gateway')
              }, {
                  path: '/oneterm/security',
                  name: 'oneterm_security',
                  meta: { title: 'oneterm.menu.security', appName: 'oneterm', icon: 'ops-oneterm-command', selectedIcon: 'ops-oneterm-command-selected', permission: ['oneterm_admin', 'Security'] },
                  component: () => import('../views/assets/security')
              }]
          },
          {
            path: '/oneterm/audit',
            name: 'oneterm_session',
            component: RouteView,
            meta: { title: 'oneterm.menu.auditCentre', appName: 'oneterm', icon: 'ops-oneterm-log', selectedIcon: 'ops-oneterm-log-selected', permission: ['oneterm_admin', 'Online_Session', 'Offline_Session', 'Login_Audit', 'Operation_Audit'] },
            redirect: '/oneterm/session/online',
            hideChildrenInMenu: false,
            children: [
              {
                  path: `/oneterm/session`,
                  name: `oneterm_session`,
                  meta: { title: 'oneterm.menu.sessionAuditing', appName: 'oneterm', disabled: true, style: 'margin-left: 12px', permission: ['oneterm_admin', 'Online_Session', 'Offline_Session'] },
              },
              {
                  path: '/oneterm/session/online',
                  name: 'oneterm_session_online',
                  meta: { title: 'oneterm.menu.onlineSession', appName: 'oneterm', icon: 'ops-oneterm-sessiononline', selectedIcon: 'ops-oneterm-sessiononline-selected', permission: ['oneterm_admin', 'Online_Session'] },
                  component: () => import('../views/session/online.vue')
              }, {
                  path: '/oneterm/session/history',
                  name: 'oneterm_session_history',
                  meta: { title: 'oneterm.menu.offlineSession', appName: 'oneterm', icon: 'ops-oneterm-sessionhistory', selectedIcon: 'ops-oneterm-sessionhistory-selected', permission: ['oneterm_admin', 'Offline_Session'] },
                  component: () => import('../views/session/history.vue')
              },
              {
                  path: `/oneterm/log`,
                  name: `oneterm_log`,
                  meta: { title: 'oneterm.menu.logAuditing', appName: 'oneterm', disabled: true, style: 'margin-left: 12px', permission: ['oneterm_admin', 'Login_Audit', 'Operation_Audit'] },
              },
              {
                  path: '/oneterm/log/login',
                  name: 'oneterm_log_login',
                  meta: { title: 'oneterm.menu.loginLog', appName: 'oneterm', icon: 'ops-oneterm-login', selectedIcon: 'ops-oneterm-login-selected', permission: ['登录日志', 'oneterm_admin', 'Login_Audit'] },
                  component: () => import('../views/log/login')
              }, {
                  path: '/oneterm/log/operation',
                  name: 'oneterm_log_operation',
                  meta: { title: 'oneterm.menu.operationLog', appName: 'oneterm', icon: 'ops-oneterm-operation', selectedIcon: 'ops-oneterm-operation-selected', permission: ['操作日志', 'oneterm_admin', 'Operation_Audit'] },
                  component: () => import('../views/log/operation')
              }
            ]
          },
            {
                path: '/oneterm/terminal',
                name: 'oneterm_terminal',
                hidden: true,
                component: () => import('../views/terminal'),
                meta: { title: '终端', keepAlive: false }
            },
            {
                path: '/oneterm/guacamole/:asset_id/:account_id/:protocol',
                name: 'oneterm_guacamole',
                hidden: true,
                component: () => import('../views/terminal/guacamoleClient.vue'),
                meta: { title: '终端', keepAlive: false }
            },
            {
                path: '/oneterm/replay/:session_id',
                name: 'oneterm_replay',
                hidden: true,
                component: () => import('../views/replay'),
                meta: { title: '回放', keepAlive: false }
            },
            {
                path: '/oneterm/replay/guacamole/:session_id',
                name: 'oneterm_replay_guacamole',
                hidden: true,
                component: () => import('../views/replay/guacamoleReplay.vue'),
                meta: { title: '回放', keepAlive: false }
            },
        ]
    }
}

export default genOnetermRoutes
