import Vue from 'vue'
import store from '@/store'

/**
 * Action permission directive
 * Usage:
 *  - Use v-action:[method] on components that require action-level permission control, e.g.:
 *    <i-button v-action:add>Add User</i-button>
 *    <a-button v-action:delete>Delete User</a-button>
 *    <a v-action:edit @click="edit(record)">Edit</a>
 *
 *  - If the current user does not have permission, the component using this directive will be hidden.
 *  - If the backend permission model is different from the pro model, just modify the permission filtering logic here.
 *
 *  @see https://github.com/sendya/ant-design-pro-vue/pull/53
 */
const action = Vue.directive('action', {
  inserted: function (el, binding, vnode) {
    const actionName = binding.arg
    const roles = store.getters.roles
    const elVal = vnode.context.$route.meta.permission
    const permissionId = elVal instanceof String && [elVal] || elVal || []
    roles.permissions.forEach(p => {
      if (!permissionId.includes(p.permissionId)) {
        return
      }
      if (p.actionList && !p.actionList.includes(actionName)) {
        el.parentNode && el.parentNode.removeChild(el) || (el.style.display = 'none')
      }
    })
  }
})

export default action
