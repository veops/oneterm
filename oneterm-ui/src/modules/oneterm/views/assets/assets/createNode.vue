<template>
  <CustomDrawer
    :closable="true"
    :visible="visible"
    width="1000px"
    :title="title"
    @close="visible = false"
  >
    <p>
      <strong>{{ $t(`oneterm.baseInfo`) }}</strong>
    </p>
    <a-form-model
      ref="baseForm"
      :model="baseForm"
      :rules="baseRules"
      :label-col="{ span: 5 }"
      :wrapper-col="{ span: 16 }"
    >
      <a-form-model-item :label="$t('oneterm.assetList.catalogName')" prop="name">
        <a-input v-model="baseForm.name" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
      <a-form-model-item :label="$t(`oneterm.catalog`)" prop="parent_id">
        <treeselect
          class="custom-treeselect custom-treeselect-white"
          :style="{
            '--custom-height': '32px',
            lineHeight: '32px',
          }"
          v-model="baseForm.parent_id"
          :multiple="false"
          :clearable="true"
          searchable
          :options="nodeList"
          :placeholder="`${$t(`placeholder2`)}`"
          :normalizer="
            (node) => {
              return {
                id: node.id,
                label: node.name,
              }
            }
          "
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
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.comment')" prop="comment">
        <a-textarea v-model="baseForm.comment" :placeholder="`${$t(`placeholder1`)}`" />
      </a-form-model-item>
    </a-form-model>
    <!-- <p>
      <strong>{{ $t(`oneterm.assetList.cmdbSync`) }}</strong>
    </p>
    <a-form-model ref="syncForm" :model="syncForm" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
      <a-form-model-item
        :label="$t('oneterm.cmdbType')"
        prop="type_id"
        :style="{ display: 'flex', alignItems: 'center' }"
      >
        <CMDBTypeSelect v-model="syncForm.type_id" selectType="ci_type" @change="changeTypeId" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.fieldMap')">
        <div v-for="item in fieldMap" :key="item.id">
          <div class="cmdb-radio-slot-field">
            <div class="slot-field1">
              <span>*</span>
              <a-input disabled size="small" :style="{ width: '200px' }" v-model="item['attribute'].label" />
            </div>
            <div class="slot-field2" :style="{ marginLeft: '150px' }">
              <treeselect
                class="custom-treeselect custom-treeselect-white"
                :style="{
                  '--custom-height': '24px',
                  lineHeight: '24px',
                  width: '250px',
                }"
                v-model="item.field_name"
                :multiple="false"
                :clearable="true"
                searchable
                :options="attributes"
                :placeholder="`${$t(`placeholder2`)}`"
                :normalizer="
                  (node) => {
                    return {
                      id: node.name,
                      label: node.alias || node.name,
                    }
                  }
                "
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
            </div>
          </div>
          <span v-if="item.error" style="color: red">{{ `${$t(`placeholder2`)}` }}</span>
        </div>
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.filter')" class="cmdb-value-filter">
        <FilterComp
          ref="filterComp"
          :isDropdown="false"
          :canSearchPreferenceAttrList="attributes"
          @setExpFromFilter="setExpFromFilter"
          :expression="filterExp ? `q=${filterExp}` : ''"
        />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.assetList.sync')" prop="enable">
        <a-switch v-model="syncForm.enable" />
      </a-form-model-item>
      <a-form-model-item :label="$t('oneterm.assetList.frequency')" prop="frequency">
        <a-input-number :min="0" v-model="syncForm.frequency" />{{ $t('hour') }}
      </a-form-model-item>
    </a-form-model> -->
    <p>
      <strong>{{ $t(`oneterm.protocol`) }}</strong>
    </p>
    <Protocol ref="protocol" />
    <p>
      <strong>{{ $t(`oneterm.accountAuthorization`) }}</strong>
    </p>
    <Account ref="account" />
    <p>
      <strong>{{ $t(`oneterm.accessRestrictions`) }}</strong>
    </p>
    <AccessAuth ref="accessAuth" />
    <div class="custom-drawer-bottom-action">
      <a-button
        :loading="loading"
        @click="
          () => {
            visible = false
          }
        "
      >{{ $t(`cancel`) }}</a-button
      >
      <a-button :loading="loading" @click="handleSubmit" type="primary">{{ $t(`confirm`) }}</a-button>
    </div>
  </CustomDrawer>
</template>

<script>
import CMDBTypeSelect from '../../../components/cmdbTypeSelect'
import { getCITypeAttributesById } from '../../../api/otherModules'
import FilterComp from '@/components/CMDBFilterComp'
import Protocol from './protocol.vue'
import Account from './account.vue'
import AccessAuth from './accessAuth.vue'
import { getNodeList, postNode, putNodeById } from '../../../api/node'

export default {
  name: 'CreateNode',
  components: { CMDBTypeSelect, FilterComp, Protocol, Account, AccessAuth },
  data() {
    return {
      visible: false,
      type: 'create',
      nodeId: null,
      loading: false,
      baseForm: {
        name: '',
        parent_id: undefined,
        comment: '',
      },
      baseRules: {
        name: [{ required: true, message: `${this.$t(`placeholder1`)}` }],
      },
      syncForm: {
        type_id: undefined,
        enable: true,
        frequency: undefined,
      },
      // fieldMap
      fieldMap: [
        {
          field_name: undefined,
          attribute: { value: 'name', label: this.$t('oneterm.name') },
        },
        {
          field_name: undefined,
          attribute: { value: 'ip', label: 'IP' },
        },
      ],
      fieldMapObj: {
        ip: 'IP',
        name: this.$t('oneterm.name'),
      },
      attributes: [],
      filterExp: '',
      nodeList: [],
    }
  },
  computed: {
    title() {
      if (this.type === 'create') {
        return this.$t(`oneterm.assetList.createCatalog`)
      }
      return this.$t(`oneterm.assetList.editCatalog`)
    },
  },
  mounted() {},
  methods: {
    setNode(node, type) {
      this.visible = true
      this.type = type
      this.$nextTick(async () => {
        const params = {}
        if (node?.id) {
          params.no_self_child = node.id
        }
        getNodeList(params).then((res) => {
          this.nodeList = res?.data?.list || []
        })
        console.log(node)
        const {
          id = null,
          name = '',
          comment = '',
          parent_id,
          sync = {},
          gateway_id = undefined,
          protocols = [],
          authorization = {},
          access_auth = {},
        } = node ?? {}
        const { type_id = undefined, enable = true, frequency = undefined, filters = '', mapping = {} } = sync
        this.nodeId = id
        this.baseForm = {
          name,
          parent_id: parent_id || undefined,
          comment,
        }
        this.syncForm = {
          type_id,
          enable,
          frequency,
        }
        this.$nextTick(() => {
          this.fieldMap =
            JSON.stringify(mapping) === '{}'
              ? [
                  {
                    field_name: undefined,
                    attribute: { value: 'name', label: this.$t('oneterm.name') },
                  },
                  {
                    field_name: undefined,
                    attribute: { value: 'ip', label: 'IP' },
                  },
                ]
              : Object.keys(mapping).map((key) => {
                  return {
                    field_name: mapping[key],
                    attribute: { value: key, label: this.fieldMapObj[key] },
                  }
                })
        })
        this.filterExp = filters
        // this.$nextTick(() => {
        //   this.$refs.filterComp.visibleChange(true, false)
        // })
        this.$refs.protocol.setValues({ gateway_id, protocols })
        this.$refs.account.setValues({ authorization })
        this.$refs.accessAuth.setValues(access_auth)
      })
    },
    async changeTypeId(id) {
      this.fieldMap = [
        {
          field_name: undefined,
          attribute: { value: 'name', label: this.$t('oneterm.name') },
        },
        {
          field_name: undefined,
          attribute: { value: 'ip', label: 'IP' },
        },
      ]
      if (id) {
        await getCITypeAttributesById(id).then((res) => {
          const { attributes } = res
          this.attributes = attributes
        })
      } else {
        this.attributes = []
      }
    },
    // setExpFromFilter(filterExp) {
    //   if (filterExp) {
    //     this.filterExp = `${filterExp}`
    //   } else {
    //     this.filterExp = ''
    //   }
    // },
    handleSubmit() {
      this.$refs.baseForm.validate((valid) => {
        if (valid) {
          const { name, parent_id, comment } = this.baseForm
          // const { type_id, enable, frequency } = this.syncForm
          // this.$refs.filterComp.handleSubmit()
          // const mapping = {}
          // let flag = true
          // this.fieldMap.forEach((field) => {
          //   if (!field.field_name) {
          //     this.$set(field, 'error', true)
          //     field.error = true
          //     flag = false
          //   } else {
          //     this.$set(field, 'error', false)
          //     field.error = false
          //     mapping[field.attribute.value] = field.field_name
          //   }
          // })
          // if (type_id && !flag) {
          //   return
          // }
          const { gateway_id, protocols } = this.$refs.protocol.getValues()
          const { authorization } = this.$refs.account.getValues()
          const access_auth = this.$refs.accessAuth.getValues()
          const params = {
            name,
            comment,
            parent_id: parent_id ?? 0,
            // sync: {
            //   enable,
            //   filters: this.filterExp,
            //   frequency,
            //   mapping,
            //   type_id,
            // },
            protocols,
            gateway_id,
            authorization,
            access_auth,
          }
          console.log(params)
          this.loading = true
          if (this.nodeId) {
            putNodeById(this.nodeId, { ...params, id: this.nodeId })
              .then((res) => {
                this.$message.success(this.$t('editSuccess'))
                this.$emit('submitNode')
                this.visible = false
              })
              .finally(() => {
                this.loading = false
              })
          } else {
            postNode(params)
              .then((res) => {
                this.$message.success(this.$t('createSuccess'))
                this.$emit('submitNode')
                this.visible = false
              })
              .finally(() => {
                this.loading = false
              })
          }
        }
      })
    },
  },
}
</script>

<style lang="less" scoped>
.cmdb-radio-slot-field {
  display: flex;
  align-items: center;
  .slot-field1 {
    position: relative;
    > span {
      color: red;
      position: absolute;
      left: 72px;
      z-index: 1;
    }
    &::before {
      content: '';
      position: absolute;
      width: 10px;
      height: 10px;
      background-color: #e1efff;
      border-radius: 50%;
      right: -20px;
      top: 50%;
      transform: translateY(-50%);
    }
    &::after {
      content: '';
      position: absolute;
      width: 4px;
      height: 4px;
      background-color: #2f54eb;
      border-radius: 50%;
      right: -17px;
      top: 50%;
      transform: translateY(-50%);
    }
  }
  .slot-field2 {
    position: relative;
    &::before {
      content: '';
      position: absolute;
      width: 120px;
      height: 1px;
      background-color: #cacdd9;
      left: -130px;
      top: 50%;
      transform: translateY(-50%);
    }
    &::after {
      content: '';
      position: absolute;
      width: 0;
      height: 0;
      border-width: 5px;
      border-style: solid;
      border-color: transparent transparent transparent #cacdd9;
      left: -15px;
      top: 50%;
      transform: translateY(-50%);
    }
  }
}
</style>
<style lang="less">
.asset-create-node-container {
  .ant-form-item {
    margin-bottom: 8px;
  }
}

.cmdb-value-filter {
  .ant-form-item-control {
    line-height: 24px;
  }
  .table-filter-add {
    line-height: 40px;
  }
}

.cmdb-radio-slot-field {
  .ant-input[disabled] {
    background-color: #fff;
    cursor: default;
    color: rgba(0, 0, 0, 0.65);
    padding-left: 80px;
  }
}
</style>
