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
              meta: { title: 'dashboard', icon: 'ops-oneterm-dashboard', selectedIcon: 'ops-oneterm-dashboard-selected', keepAlive: false }
          },
          {
              path: '/oneterm/workstation',
              name: 'onterm_work_station',
              component: () => import('../views/workStation'),
              meta: {
                title: 'oneterm.menu.workStation', icon: 'ops-oneterm-workstation', selectedIcon: 'ops-oneterm-workstation-selected', keepAlive: false
              }
          },
          {
              path: '/oneterm/publickey',
              name: 'onterm_public_key',
              component: () => import('../views/publicKey'),
              meta: { title: 'oneterm.menu.publicKey', keepAlive: false, icon: 'ops-oneterm-publickey', selectedIcon: 'ops-oneterm-publickey-selected' }
          },
          {
              path: '/oneterm/assets',
              name: 'oneterm_assets',
              component: RouteView,
              meta: { title: 'oneterm.menu.assetManagement', icon: 'ops-oneterm-assets', selectedIcon: 'ops-oneterm-assets-selected' },
              redirect: '/oneterm/assets/assets',
              children: [{
                  path: '/oneterm/assetlist',
                  name: 'oneterm_asset_list',
                  meta: { title: 'oneterm.menu.assets', icon: 'ops-oneterm-assetlist', selectedIcon: 'ops-oneterm-assetlist-selected' },
                  component: () => import('../views/assets/assets')
              }, {
                path: '/oneterm/account',
                name: 'oneterm_account',
                meta: { title: 'oneterm.menu.accounts', icon: 'ops-oneterm-account', selectedIcon: 'ops-oneterm-account-selected' },
                component: () => import('../views/assets/account')
              }, {
                  path: '/oneterm/gateway',
                  name: 'oneterm_gateway',
                  meta: { title: 'oneterm.menu.gateways', icon: 'ops-oneterm-gateway', selectedIcon: 'ops-oneterm-gateway-selected' },
                  component: () => import('../views/assets/gateway')
              }, {
                  path: '/oneterm/security',
                  name: 'oneterm_security',
                  meta: { title: 'oneterm.menu.security', icon: 'ops-oneterm-command', selectedIcon: 'ops-oneterm-command-selected' },
                  component: () => import('../views/assets/security')
              }]
          },
          {
            path: '/oneterm/audit',
            name: 'oneterm_session',
            component: RouteView,
            meta: { title: 'oneterm.menu.auditCentre', icon: 'ops-oneterm-log', selectedIcon: 'ops-oneterm-log-selected' },
            redirect: '/oneterm/session/online',
            hideChildrenInMenu: false,
            children: [
              {
                  path: `/oneterm/session`,
                  name: `oneterm_session`,
                  meta: { title: 'oneterm.menu.sessionAuditing', disabled: true, style: 'margin-left: 12px' },
              },
              {
                  path: '/oneterm/session/online',
                  name: 'oneterm_session_online',
                  meta: { title: 'oneterm.menu.onlineSession', icon: 'ops-oneterm-sessiononline', selectedIcon: 'ops-oneterm-sessiononline-selected' },
                  component: () => import('../views/session/online.vue')
              }, {
                  path: '/oneterm/session/history',
                  name: 'oneterm_session_history',
                  meta: { title: 'oneterm.menu.offlineSession', icon: 'ops-oneterm-sessionhistory', selectedIcon: 'ops-oneterm-sessionhistory-selected' },
                  component: () => import('../views/session/history.vue')
              },
              {
                  path: `/oneterm/log`,
                  name: `oneterm_log`,
                  meta: { title: 'oneterm.menu.logAuditing', disabled: true, style: 'margin-left: 12px' },
              },
              {
                  path: '/oneterm/log/login',
                  name: 'oneterm_log_login',
                  meta: { title: 'oneterm.menu.loginLog', icon: 'ops-oneterm-login', selectedIcon: 'ops-oneterm-login-selected' },
                  component: () => import('../views/log/login')
              }, {
                  path: '/oneterm/log/operation',
                  name: 'oneterm_log_operation',
                  meta: { title: 'oneterm.menu.operationLog', icon: 'ops-oneterm-operation', selectedIcon: 'ops-oneterm-operation-selected' },
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
