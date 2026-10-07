<template>
  <a-modal
    :visible="visible"
    :width="800"
    :footer="null"
    @cancel="handleCancel"
  >
    <a-tabs v-model="tabKey">
      <a-tab-pane v-if="showAccessPermission" key="accessPermission" :tab="$t('oneterm.assetList.accessPermission')">
        <PermissionForm
          ref="permissionFormRef"
          :dataType="dataType"
          :ids="ids"
          @cancel="handleCancel"
        />
      </a-tab-pane>
      <a-tab-pane
        v-if="showACLConfig"
        key="operationPermissions"
        :tab="$t(grantTitle)"
      >
        <ACLTable
          :tableData="aclTableData"
          :resourceId="resourceId"
          :columns="permissionColumns"
        />
        <a-space>
          <span class="grant-button" @click="openGrantUserModal('depart')">{{ $t('oneterm.assetList.grantUserOrDep') }}</span>
          <span class="grant-button" @click="openGrantUserModal('role')">{{ $t('oneterm.assetList.grantRole') }}</span>
        </a-space>
      </a-tab-pane>
    </a-tabs>

    <GrantUserModal
      ref="grantUserModalRef"
      @handleOk="grantUserModalOk"
    />
  </a-modal>
</template>

<script>
import _ from 'lodash'
import { mapState } from 'vuex'
import { getResourcePerms } from '@/modules/acl/api/permission'
import { searchResourceType } from '@/modules/acl/api/resource'
import { emptyPermissions, permissionColumns } from './permissions'

import PermissionForm from './permissionForm.vue'
import ACLTable from './aclTable.vue'
import GrantUserModal from './grantUserModal.vue'

export default {
  name: 'GrantModal',
  components: {
    PermissionForm,
    ACLTable,
    GrantUserModal
  },
  data() {
    return {
      tabKey: 'accessPermission',
      visible: false,
      resourceId: '', // acl resource id
      aclTableData: [], // acl data
      ids: [], // id list
      dataType: 'node',
      showACLConfig: true,
      permissionColumns: [],
      requestGeneration: 0,
    }
  },
  computed: {
    showAccessPermission() {
      return ['node', 'asset', 'account'].includes(this.dataType)
    },
    ...mapState({
      allEmployees: (state) => state.user.allEmployees,
      allDepartments: (state) => state.user.allDepartments,
    }),
    grantTitle() {
      switch (this.dataType) {
        case 'node':
          return 'oneterm.assetList.folderOperationPermissions'
        case 'asset':
          return 'oneterm.assetList.assetOperationPermissions'
        case 'account':
          return 'oneterm.assetList.accountOperationPermissions'
        default:
          return 'oneterm.assetList.operationPermissions'
      }
    }
  },
  methods: {
    async open({
      type,
      ids,
      resourceId,
    }) {
      const generation = ++this.requestGeneration
      this.showACLConfig = type === 'node' || (type !== 'node' && ids.length === 1)
      this.ids = ids
      this.dataType = type
      this.resourceId = resourceId ?? ''
      this.tabKey = this.showAccessPermission ? 'accessPermission' : 'operationPermissions'

      let aclTableData = []
      if (this.showACLConfig) {
        const definitions = await searchResourceType({ app_id: 'oneterm', q: type, page_size: 100 })
        if (generation !== this.requestGeneration) return
        const resourceType = (definitions?.groups || []).find((item) => item.name === type)
        const actions = resourceType ? (definitions?.id2perms?.[resourceType.id] || []).map((item) => item.name) : []
        this.permissionColumns = permissionColumns(actions)
        const aclPerms = await getResourcePerms(resourceId, {
          need_users: 0
        })
        if (generation !== this.requestGeneration) return
        const permsKeys = Object.keys(aclPerms)
        if (permsKeys.length) {
          aclTableData = permsKeys.map((key) => {
            const perms = aclPerms?.[key]?.perms

            const data = {
              name: key,
              rid: perms?.[0]?.rid || '',
              permissions: emptyPermissions(this.permissionColumns)
            }
            perms.forEach((item) => {
              if (Object.prototype.hasOwnProperty.call(data.permissions, item.name)) {
                data.permissions[item.name] = true
              }
            })
            return data
          })
        }
      }
      this.aclTableData = aclTableData
      this.visible = true
    },

    handleCancel() {
      this.requestGeneration += 1
      this.visible = false
      if (this.$refs.permissionFormRef) {
        this.$refs.permissionFormRef.resetFields()
      }
      this.resourceId = ''
      this.aclTableData = []
      this.ids = []
      this.dataType = 'node'
      this.showACLConfig = true
      this.permissionColumns = []
    },

    openGrantUserModal(type) {
      this.$refs.grantUserModalRef.open(type)
    },

    grantUserModalOk(params, type) {
      let addTableData = []
      if (type === 'depart') {
        addTableData = [
          ...params.department.map((rid) => {
            const _find = this.allDepartments.find((dep) => dep.acl_rid === rid)
            return { rid, name: _find?.department_name ?? rid }
          }),
          ...params.user.map((rid) => {
            const _find = this.allEmployees.find((dep) => dep.acl_rid === rid)
            return { rid, name: _find?.nickname ?? rid }
          }),
        ]
      }
      if (type === 'role') {
        addTableData = [
          ...params.map((role) => {
            return { rid: role.id, name: role.name }
          }),
        ]
      }

      addTableData.forEach((item) => {
        item.permissions = emptyPermissions(this.permissionColumns)
      })

      const newTableData = _.uniqBy(
        [
          ...this.aclTableData,
          ...addTableData
        ],
        'rid'
      )
      this.$set(this, 'aclTableData', newTableData)
    }
  }
}
</script>

<style lang="less" scoped>
.auth-node-title {
  border-left: 4px solid #2f54eb;
  padding-left: 10px;
  margin-bottom: 18px;
}
.grant-button {
  padding: 6px 8px;
  color: @primary-color;
  background-color: @primary-color_5;
  border-radius: 2px;
  cursor: pointer;
  margin: 15px 0;
  display: inline-block;
  transition: all 0.3s;
  &:hover {
    box-shadow: 2px 3px 4px @primary-color_5;
  }
}
</style>
