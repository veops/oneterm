<template>
  <a-row class="form-account">
    <a-col v-bind="colSpan">
      <!-- Web资产特殊提示 -->
      <div v-if="isWebAsset" class="web-asset-notice">
        <a-alert
          type="info"
          show-icon
          :message="$t('oneterm.webAsset.authNotice')"
          :description="$t('oneterm.webAsset.authDescription')"
        />
        <div class="web-asset-auth-form">
          <h4>{{ $t('oneterm.webAsset.webConfigAuth') }}</h4>
          <vxe-table
            ref="xTable"
            size="mini"
            :data="authList"
            :column-config="{ width: 200 }"
            :min-height="110"
          >
            <vxe-column field="webAsset" :title="$t('oneterm.webAsset.target')" width="190">
              <template #default="{ row }">
                <span class="web-asset-label">{{ $t('oneterm.webAsset.entireAsset') }}</span>
              </template>
            </vxe-column>
            <vxe-column field="grantUser" :title="$t('oneterm.assetList.grantRole')" width="190">
              <template #default="{ row }">
                <EmployeeTreeSelect
                  v-model="row.rids"
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
              </template>
            </vxe-column>
            <vxe-column field="permissions" :title="$t('oneterm.assetList.operationPermissions')" width="230">
              <template #default="{ row }">
                <PermissionCheckbox
                  :value="row.permissions"
                  @change="(e) => handlePermissionChange(row, e)"
                />
              </template>
            </vxe-column>
            <vxe-column field="operate" :title="$t('oneterm.assetList.operation')" width="120">
              <template #default="{ row, rowIndex }">
                <a-space>
                  <a
                    v-if="authList && authList.length > 1"
                    style="color:red"
                    @click="deleteCount(row.id)"
                  >
                    <ops-icon type="veops-delete" />
                  </a>
                  <a
                    v-if="rowIndex === authList.length - 1 && authList.length < 10"
                    @click="addCount"
                  >
                    <a-icon type="plus-circle" />
                  </a>
                </a-space>
              </template>
            </vxe-column>
          </vxe-table>
        </div>
      </div>

      <!-- 传统资产账号选择 -->
      <vxe-table
        v-else
        ref="xTable"
        size="mini"
        :data="authList"
        :column-config="{ width: 200 }"
        :min-height="110"
      >
        <vxe-column field="account" :title="$t('oneterm.account')" width="190">
          <template #default="{ row }">
            <a-select
              v-model="row.account"
              showSearch
              :style="{
                width: '180px',
              }"
              :placeholder="$t('placeholder2')"
              optionFilterProp="title"
              allowClear
            >
              <a-select-option
                v-for="(node, nodeIndex) in accountList"
                :key="node.id + nodeIndex"
                :value="node.id"
                :title="node.name"
              >
                <a-tooltip :title="node.toolTip">
                  {{ node.name }}
                  <span v-if="node.account" class="select-option-name">({{ node.account }})</span>
                </a-tooltip>
              </a-select-option>
            </a-select>
          </template>
        </vxe-column>
        <vxe-column field="grantUser" :title="$t('oneterm.assetList.grantRole')" width="190">
          <template #default="{ row }">
            <EmployeeTreeSelect
              v-model="row.rids"
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
          </template>
        </vxe-column>
        <vxe-column field="permissions" :title="$t('oneterm.assetList.operationPermissions')" width="230">
          <template #default="{ row }">
            <PermissionCheckbox
              :value="row.permissions"
              @change="(e) => handlePermissionChange(row, e)"
            />
          </template>
        </vxe-column>
        <vxe-column field="operate" :title="$t('oneterm.assetList.operation')" width="120">
          <template #default="{ row, rowIndex }">
            <a-space>
              <a
                v-if="authList && authList.length > 1"
                style="color:red"
                @click="deleteCount(row.id)"
              >
                <ops-icon type="veops-delete" />
              </a>
              <a
                v-if="rowIndex === authList.length - 1 && authList.length < 10"
                @click="addCount"
              >
                <a-icon type="plus-circle" />
              </a>
            </a-space>
          </template>
        </vxe-column>
      </vxe-table>
    </a-col>
  </a-row>
</template>

<script>
import { v4 as uuidv4 } from 'uuid'
import { getAccountList } from '@/modules/oneterm/api/account'
import { searchRole } from '@/modules/acl/api/role'
import { getConfig } from '@/modules/oneterm/api/config'
import { PERMISSION_TYPE } from '@/modules/oneterm/views/systemSettings/accessControl/constants.js'

import EmployeeTreeSelect from '@/views/setting/components/employeeTreeSelect.vue'
import PermissionCheckbox from '@/modules/oneterm/views/systemSettings/accessControl/permissionCheckbox.vue'

const DEFAULT_PERMISSIONS = Object.values(PERMISSION_TYPE).reduce((config, key) => {
  config[key] = false
  return config
}, {})

export default {
  name: 'Account',
  components: {
    EmployeeTreeSelect,
    PermissionCheckbox
  },
  props: {
    colSpan: {
      type: Object,
      default: () => ({
        span: 17,
        offset: 4,
      }),
    },
  },
  data() {
    return {
      accountList: [],
      authList: [{
        id: uuidv4(),
        account: undefined,
        rids: undefined,
        permissions: { ...DEFAULT_PERMISSIONS }
      }],
      visualRoleList: []
    }
  },
  created() {
    this.initDefaultPermissions()
    this.getRoleList()
    this.getAccountList()
  },
  methods: {
    initDefaultPermissions() {
      getConfig({
        info: true
      }).then((res) => {
        const default_permissions = res?.data?.default_permissions
        Object.keys(DEFAULT_PERMISSIONS).forEach((key) => {
          DEFAULT_PERMISSIONS[key] = default_permissions?.[key] ?? DEFAULT_PERMISSIONS[key]
        })
        this.authList.forEach((item) => {
          item.permissions = { ...DEFAULT_PERMISSIONS }
        })
      })
    },

    async getRoleList() {
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

    async getAccountList() {
      const res = await getAccountList({ page_index: 1 })
      const accountList = res?.data?.list || []
      accountList.forEach((item) => {
        item.toolTip = item.name + (item.account ? '(item.account)' : '')
      })
      this.accountList = accountList
    },

    addCount() {
      this.authList.push({
        id: uuidv4(),
        account: undefined,
        rids: undefined,
        permissions: { ...DEFAULT_PERMISSIONS }
      })
    },
    deleteCount(id) {
      const index = this.authList.findIndex((item) => item.id === id)
      if (index !== -1) {
        this.authList.splice(index, 1)
      }
    },

    getValues() {
      const authorization = {}

      this.authList
        .filter((auth) => auth.account)
        .forEach((auth) => {
          const rids = (auth?.rids || []).map((r) => Number(r.split('-')[1]))
          const authorizationItem = {
            rids,
            permissions: auth.permissions
          }
          if (auth?.rule_id) {
            authorizationItem.rule_id = auth.rule_id
          }
          authorization[auth.account] = authorizationItem
        })
      return { authorization }
    },

    setValues({ authorization = {} }) {
      const authorizationList = Object.entries(authorization || {})
      if (authorizationList.length) {
        this.authList = authorizationList.map(([key, value]) => {
          return {
            id: uuidv4(),
            account: Number(key),
            rule_id: value?.rule_id ?? undefined,
            rids: (value?.rids || []).map((r) => `employee-${r}`),
            permissions: value.permissions
          }
        })
      } else {
        this.authList = [{
          id: uuidv4(),
          account: undefined,
          rids: undefined,
          permissions: { ...DEFAULT_PERMISSIONS }
        }]
      }
    },
  },
}
</script>

<style lang="less" scoped>
.form-account {
  /deep/ .ant-checkbox-wrapper {
    margin-right: 0px;
    width: 48%;
  }
}
.select-option-name {
  font-size: 12px;
  color: #A5A9BC;
}
</style>
