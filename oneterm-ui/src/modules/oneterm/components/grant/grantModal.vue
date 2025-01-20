<template>
  <a-modal
    :visible="visible"
    :width="800"
    @cancel="handleCancel"
    @ok="handleOk"
  >
    <template v-if="showACLConfig">
      <div class="auth-node-title">
        {{ $t(grantTitle) }}
      </div>
      <ACLTable
        :tableData="aclTableData"
        :resourceId="resourceId"
      />
      <a-space>
        <span class="grant-button" @click="openGrantUserModal('depart')">{{ $t('oneterm.assetList.grantUserOrDep') }}</span>
        <span class="grant-button" @click="openGrantUserModal('role')">{{ $t('oneterm.assetList.grantRole') }}</span>
      </a-space>
    </template>

    <div class="auth-node-title">
      {{ $t('oneterm.assetList.grantLogin') }}
    </div>
    <EmployeeTreeSelect
      v-model="rids"
      multiple
      :idType="2"
      departmentKey="acl_rid"
      employeeKey="acl_rid"
      :placeholder="`${$t(`placeholder2`)}`"
      class="custom-treeselect custom-treeselect-white"
      :style="{
        '--custom-height': '32px',
        lineHeight: '32px',
        '--custom-multiple-lineHeight': '18px',
      }"
      :limit="1"
      :otherOptions="visualRoleList"
    />

    <GrantUserModal
      ref="grantUserModalRef"
      @handleOk="grantUserModalOk"
    />
  </a-modal>
</template>

<script>
import _ from 'lodash'
import { mapState } from 'vuex'
import { getAuth, postAuth } from '@/modules/oneterm/api/authorization.js'
import { getResourcePerms } from '@/modules/acl/api/permission'
import { searchRole } from '@/modules/acl/api/role'
import EmployeeTreeSelect from '@/views/setting/components/employeeTreeSelect.vue'
import ACLTable from './aclTable.vue'
import GrantUserModal from './grantUserModal.vue'

export default {
  name: 'GrantModal',
  components: {
    EmployeeTreeSelect,
    ACLTable,
    GrantUserModal
  },
  data() {
    return {
      visible: false,
      rids: [], // 登录权限
      resourceId: '', // acl resource id
      aclTableData: [], // acl data
      ids: [], // id 列表
      dataType: 'node',
      showACLConfig: true, // 是否展示acl 配置
      visualRoleList: [], // 虚拟角色
    }
  },
  computed: {
    ...mapState({
      allEmployees: (state) => state.user.allEmployees,
      allDepartments: (state) => state.user.allDepartments,
    }),
    grantTitle() {
      switch (this.dataType) {
        case 'node':
          return 'oneterm.assetList.grantCatalog'
        case 'asset':
          return 'oneterm.assetList.grantAsset'
        case 'account':
          return 'oneterm.assetList.grantAccount'
        default:
          return ''
      }
    }
  },
  methods: {
    async open({
      type,
      ids,
      resourceId,
    }) {
      this.showACLConfig = type === 'node' || (type !== 'node' && ids.length === 1)
      this.ids = ids
      this.dataType = type
      this.resourceId = resourceId || ''

      this.loadRoles()

      let aclTableData = []
      if (this.showACLConfig) {
        const aclPerms = await getResourcePerms(resourceId, {
          need_users: 0
        })
        const permsKeys = Object.keys(aclPerms)
        if (permsKeys.length) {
          aclTableData = permsKeys.map((key) => {
            const perms = aclPerms?.[key]?.perms

            const data = {
              name: key,
              rid: perms?.[0]?.rid || '',
              read: false,
              write: false,
              delete: false,
              grant: false
            }
            perms.forEach((item) => {
              if (data?.[item.name] !== undefined) {
                data[item.name] = true
              }
            })
            return data
          })
        }
      }
      this.aclTableData = aclTableData

      const getAuthParams = {
        page_index: 1,
        page_size: 9999,
      }
      switch (type) {
        case 'node':
          getAuthParams.node_id = ids[0]
          break
        case 'asset':
          if (ids.length === 1) {
            getAuthParams.asset_id = ids[0]
          }
          break
        case 'account':
          if (ids.length === 1) {
            getAuthParams.account_id = ids[0]
          }
          break
        default:
          break
      }

      let rids = []
      if (
        getAuthParams.node_id ||
        getAuthParams.asset_id ||
        getAuthParams.account_id
      ) {
        const res = await getAuth(getAuthParams)

        let newRids = []
        if (res?.data?.list?.length) {
          res.data.list.forEach((item) => {
            newRids.push(...(item?.rids || []))
          })
        }
        newRids = _.uniq(newRids)
        if (newRids?.length) {
          rids = newRids.map((id) => `employee-${id}`)
        }
      }

      this.rids = rids
      this.visible = true
    },

    async loadRoles() {
      const res = await searchRole({ page_size: 9999, page: 1, app_id: 'oneterm', user_role: 0, user_only: 0, is_all: true })

      const visualRoleList = []
      const roleList = (res?.roles || []).filter((item) => !/_virtual$/.test(item.name))

      if (roleList.length) {
        visualRoleList.push({
          acl_rid: -100,
          department_name: this.$t('acl.visualRole'),
          sub_departments: [],
          employees: roleList.map((item) => {
            return {
              nickname: item.name,
              acl_rid: item.id
            }
          })
        })
      }

      this.$set(this, 'visualRoleList', visualRoleList)
    },

    handleCancel() {
      this.rids = []
      this.visible = false
      this.resourceId = ''
      this.aclTableData = []
      this.ids = []
      this.dataType = 'node'
      this.showACLConfig = true
    },

    handleOk() {
      const rids = this.rids.map((rid) => {
        return Number(rid?.split?.('-')?.[1])
      })

      switch (this.dataType) {
        case 'node':
          postAuth({
            rids,
            node_id: this.ids[0]
          }).then(() => {
            this.$message.success(this.$t('updateSuccess'))
            this.handleCancel()
          })
          break
        case 'asset':
          Promise.all(
            this.ids.map((id) => {
              return postAuth({
                rids,
                asset_id: id
              })
            })
          ).then(() => {
            this.$message.success(this.$t('updateSuccess'))
            this.handleCancel()
          })
          break
        case 'account':
          Promise.all(
            this.ids.map((id) => {
              return postAuth({
                rids,
                account_id: id
              })
            })
          ).then(() => {
            this.$message.success(this.$t('updateSuccess'))
            this.handleCancel()
          })
          break
        default:
          break
      }
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
        item.read = false
        item.write = false
        item.delete = false
        item.grant = false
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
