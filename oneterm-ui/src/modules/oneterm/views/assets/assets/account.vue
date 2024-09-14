<template>
  <a-row>
    <a-col v-bind="colSpan">
      <table class="account-table">
        <tr>
          <th>{{ $t(`oneterm.name`) }}</th>
          <th>{{ $t(`oneterm.account`) }}</th>
          <th>{{ $t(`oneterm.assetList.grantUser`) }}</th>
          <th>{{ $t(`operation`) }}</th>
        </tr>
        <tr v-for="(item, index) in countList" :key="item.id">
          <td>
            <a-select
              v-model="item.name"
              showSearch
              :style="{
                width: '180px',
              }"
              :placeholder="$t('placeholder2')"
              optionFilterProp="title"
              allowClear
              @change="(value) => selectAccount(value, index)"
            >
              <a-select-option
                v-for="(node, nodeIndex) in accountList"
                :key="node.id + nodeIndex"
                :value="node.id"
                :title="node.name"
              >
                {{ node.name }}
              </a-select-option>
            </a-select>
          </td>
          <td>
            <a-select
              v-model="item.account"
              :style="{
                width: '180px',
              }"
              showSearch
              :placeholder="$t('placeholder2')"
              optionFilterProp="title"
              allowClear
              @change="(value) => selectAccount(value, index)"
            >
              <a-select-option
                v-for="(node, nodeIndex) in accountList"
                :key="node.id + nodeIndex"
                :value="node.id"
                :title="node.account"
              >
                {{ node.account }}
              </a-select-option>
            </a-select>
          </td>
          <td>
            <EmployeeTreeSelect
              v-model="item.rids"
              multiple
              :idType="2"
              departmentKey="acl_rid"
              employeeKey="acl_rid"
              :placeholder="`${$t(`placeholder2`)}`"
              class="custom-treeselect custom-treeselect-bgcAndBorder"
              :style="{
                '--custom-height': '32px',
                lineHeight: '32px',
                '--custom-bg-color': '#fff',
                '--custom-border': '1px solid #d9d9d9',
                '--custom-multiple-lineHeight': '18px',
              }"
              :limit="1"
              :otherOptions="visualRoleList"
            />
          </td>
          <td>
            <a-space :style="{ width: '60px' }">
              <a @click="addCount"><a-icon type="plus-circle"/></a>
              <a v-if="countList && countList.length > 1" @click="deleteCount(index)"><a-icon type="minus-circle"/></a>
            </a-space>
          </td>
        </tr>
      </table>
    </a-col>
  </a-row>
</template>

<script>
import { v4 as uuidv4 } from 'uuid'
import { getAccountList } from '../../../api/account'
import { searchRole } from '@/modules/acl/api/role'
import EmployeeTreeSelect from '@/views/setting/components/employeeTreeSelect.vue'

export default {
  name: 'Account',
  components: { EmployeeTreeSelect },
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
      countList: [{ id: uuidv4(), name: undefined, account: undefined, rids: undefined }],
    }
  },
  created() {
    this.loadRoles()
    getAccountList({ page_index: 1 }).then((res) => {
      this.accountList = res?.data?.list || []
    })
  },
  methods: {
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

    addCount() {
      this.countList.push({ id: uuidv4(), name: undefined, account: undefined, rids: undefined })
    },
    deleteCount(index) {
      this.countList.splice(index, 1)
    },
    selectAccount(id, index) {
      this.$nextTick(() => {
        this.$set(this.countList[index], 'name', id)
        this.$set(this.countList[index], 'account', id)
      })
    },
    getValues() {
      const authorization = {}
      this.countList
        .filter((count) => count.name)
        .forEach((count) => {
          authorization[count.name] = count?.rids?.length ? count.rids.map((r) => Number(r.split('-')[1])) : []
        })
      return { authorization }
    },
    setValues({ authorization = {} }) {
      const authorizationList = Object.entries(authorization)
      if (authorizationList.length) {
        this.countList = authorizationList.map(([acc, rids]) => {
          return { id: uuidv4(), name: Number(acc), account: Number(acc), rids: rids.map((r) => `employee-${r}`) }
        })
      } else {
        this.countList = [{ id: uuidv4(), name: undefined, account: undefined, rids: undefined }]
      }
    },
  },
}
</script>

<style lang="less" scoped>
.account-table {
  border-collapse: collapse;
  border: 1px solid #e4e7ed;
  border-spacing: 20px;
  th {
    background-color: #f0f5ff;
  }
  th,
  td {
    padding: 5px 8px;
  }
}
</style>
