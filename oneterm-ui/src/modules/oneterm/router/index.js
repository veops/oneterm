// @ts-ignore
import { BasicLayout, RouteView } from '@/layouts'
const genOnetermRoutes = () => {
    return {
        path: '/oneterm',
        name: 'oneterm',
        component: BasicLayout,
        meta: { title: 'oneterm.menu.oneterm', keepAlive: false },
        redirect: '/oneterm/workstation',
        children: [
            {
                path: '/oneterm/dashboard',
                name: 'onterm_dashboard',
                component: () => import('../views/dashboard'),
                meta: { title: 'dashboard', icon: 'ops-oneterm-dashboard', selectedIcon: 'ops-oneterm-dashboard-selected', keepAlive: false, permission: ['仪表盘', 'oneterm_admin'] }
            },
            {
                path: '/oneterm/workstation',
                name: 'onterm_work_station',
                component: () => import('../views/workStation'),
                meta: { title: 'oneterm.menu.workStation', icon: 'ops-oneterm-workstation', selectedIcon: 'ops-oneterm-workstation-selected', keepAlive: false }
            },
            {
                path: '/oneterm/assets',
                name: 'oneterm_assets',
                component: RouteView,
                meta: { title: 'oneterm.menu.assetManagement', icon: 'ops-oneterm-assets', selectedIcon: 'ops-oneterm-assets-selected', permission: ['资产管理', 'oneterm_admin'] },
                redirect: '/oneterm/assets/assets',
                children: [{
                    path: '/oneterm/assetlist',
                    name: 'oneterm_asset_list',
                    meta: { title: 'oneterm.menu.assets', icon: 'ops-oneterm-assetlist', selectedIcon: 'ops-oneterm-assetlist-selected', permission: ['资产列表', 'oneterm_admin'] },
                    component: () => import('../views/assets/assets')
                }, {
                    path: '/oneterm/gateway',
                    name: 'oneterm_gateway',
                    meta: { title: 'oneterm.menu.gateways', icon: 'ops-oneterm-gateway', selectedIcon: 'ops-oneterm-gateway-selected', permission: ['网关列表', 'oneterm_admin'] },
                    component: () => import('../views/assets/gateway')
                }, {
                    path: '/oneterm/account',
                    name: 'oneterm_account',
                    meta: { title: 'oneterm.menu.accounts', icon: 'ops-oneterm-account', selectedIcon: 'ops-oneterm-account-selected', permission: ['账号列表', 'oneterm_admin'] },
                    component: () => import('../views/assets/account')
                }, {
                    path: '/oneterm/security',
                    name: 'oneterm_security',
                    meta: { title: 'oneterm.menu.security', icon: 'ops-oneterm-command', selectedIcon: 'ops-oneterm-command-selected', permission: ['命令过滤', 'oneterm_admin'] },
                    component: () => import('../views/assets/security')
                }]
            },
            {
                path: '/oneterm/session',
                name: 'oneterm_session',
                component: RouteView,
                meta: { title: 'oneterm.menu.sessionAuditing', icon: 'ops-oneterm-session', selectedIcon: 'ops-oneterm-session-selected', permission: ['会话审计', 'oneterm_admin'] },
                redirect: '/oneterm/session/online',
                children: [{
                    path: '/oneterm/session/online',
                    name: 'oneterm_session_online',
                    meta: { title: 'oneterm.menu.onlineSession', icon: 'ops-oneterm-sessiononline', selectedIcon: 'ops-oneterm-sessiononline-selected', permission: ['在线会话', 'oneterm_admin'] },
                    component: () => import('../views/session/online.vue')
                }, {
                    path: '/oneterm/session/history',
                    name: 'oneterm_session_history',
                    meta: { title: 'oneterm.menu.offlineSession', icon: 'ops-oneterm-sessionhistory', selectedIcon: 'ops-oneterm-sessionhistory-selected', permission: ['历史会话', 'oneterm_admin'] },
                    component: () => import('../views/session/history.vue')
                }]
            },
            {
                path: '/oneterm/log',
                name: 'oneterm_log',
                component: RouteView,
                meta: { title: 'oneterm.menu.logAuditing', icon: 'ops-oneterm-log', selectedIcon: 'ops-oneterm-log-selected', permission: ['日志审计', 'oneterm_admin'] },
                redirect: '/oneterm/log/login',
                children: [{
                    path: '/oneterm/log/login',
                    name: 'oneterm_log_login',
                    meta: { title: 'oneterm.menu.loginLog', icon: 'ops-oneterm-login', selectedIcon: 'ops-oneterm-login-selected', permission: ['登录日志', 'oneterm_admin'] },
                    component: () => import('../views/log/login')
                }, {
                    path: '/oneterm/log/operation',
                    name: 'oneterm_log_operation',
                    meta: { title: 'oneterm.menu.operationLog', icon: 'ops-oneterm-operation', selectedIcon: 'ops-oneterm-operation-selected', permission: ['操作日志', 'oneterm_admin'] },
                    component: () => import('../views/log/operation')
                }]
            },
            {
                path: '/oneterm/publickey',
                name: 'onterm_public_key',
                component: () => import('../views/publicKey'),
                meta: { title: 'oneterm.menu.publicKey', keepAlive: false, icon: 'ops-oneterm-publickey', selectedIcon: 'ops-oneterm-publickey-selected', }
            },
            {
                path: '/oneterm/terminal',
                name: 'oneterm_terminal',
                hidden: true,
                component: () => import('../views/terminal'),
                meta: { title: '终端', keepAlive: false }
            },
            {
                path: '/oneterm/replay/:session_id',
                name: 'oneterm_replay',
                hidden: true,
                component: () => import('../views/replay'),
                meta: { title: '回放', icon: 'ops-itsm-servicedesk', selectedIcon: 'ops-itsm-servicedesk-selected', keepAlive: false }
            },
        ]
    }
}

export default genOnetermRoutes
