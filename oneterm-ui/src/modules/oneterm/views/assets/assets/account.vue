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
            <treeselect
              class="custom-treeselect custom-treeselect-bgcAndBorder"
              :style="{
                '--custom-height': '24px',
                lineHeight: '24px',
                '--custom-bg-color': '#fff',
                '--custom-border': '1px solid #d9d9d9',
              }"
              v-model="item.name"
              :multiple="false"
              :clearable="true"
              searchable
              :options="accountList"
              :placeholder="`${$t(`placeholder2`)}`"
              :normalizer="
                (node) => {
                  return {
                    id: node.id,
                    label: node.name,
                  }
                }
              "
              @select="(node, instanceId) => selectAccount(node, instanceId, index)"
              @input="(value, instanceId) => deselectAccount(value, instanceId, index)"
              appendToBody
              :z-index="1056"
            >
              <div
                :title="node.label"
                slot="option-label"
                slot-scope="{ node }"
                :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
              >
                {{ node.label }}
              </div>
            </treeselect>
          </td>
          <td>
            <treeselect
              class="custom-treeselect custom-treeselect-bgcAndBorder"
              :style="{
                '--custom-height': '24px',
                lineHeight: '24px',
                '--custom-bg-color': '#fff',
                '--custom-border': '1px solid #d9d9d9',
              }"
              v-model="item.account"
              :multiple="false"
              :clearable="true"
              searchable
              :options="accountList"
              :placeholder="`${$t(`placeholder2`)}`"
              :normalizer="
                (node) => {
                  return {
                    id: node.id,
                    label: node.account,
                  }
                }
              "
              @select="(node, instanceId) => selectAccount(node, instanceId, index)"
              @input="(value, instanceId) => deselectAccount(value, instanceId, index)"
              appendToBody
              :z-index="1056"
            >
              <div
                :title="node.label"
                slot="option-label"
                slot-scope="{ node }"
                :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
              >
                {{ node.label }}
              </div>
            </treeselect>
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
                '--custom-height': '24px',
                lineHeight: '24px',
                '--custom-bg-color': '#fff',
                '--custom-border': '1px solid #d9d9d9',
              }"
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
    getAccountList({ page_index: 1 }).then((res) => {
      this.accountList = res?.data?.list || []
    })
  },
  methods: {
    addCount() {
      this.countList.push({ id: uuidv4(), name: undefined, account: undefined, rids: undefined })
    },
    deleteCount(index) {
      this.countList.splice(index, 1)
    },
    selectAccount(node, instanceId, index) {
      const { id } = node
      this.$nextTick(() => {
        this.$set(this.countList, index, { id, name: id, account: id })
      })
    },
    deselectAccount(value, instanceId, index) {
      if (!value) {
        this.$nextTick(() => {
          this.$set(this.countList, index, { id: uuidv4(), name: undefined, account: undefined })
        })
      }
    },
    getValues() {
      const authorization = {}
      this.countList
        .filter((count) => typeof count.id === 'number')
        .forEach((count) => {
          authorization[count.id] = count?.rids?.length ? count.rids.map((r) => Number(r.split('-')[1])) : []
        })
      return { authorization }
    },
    setValues({ authorization = {} }) {
      const authorizationList = Object.entries(authorization)
      if (authorizationList.length) {
        this.countList = authorizationList.map(([acc, rids]) => {
          return { id: Number(acc), name: Number(acc), account: Number(acc), rids: rids.map((r) => `employee-${r}`) }
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
