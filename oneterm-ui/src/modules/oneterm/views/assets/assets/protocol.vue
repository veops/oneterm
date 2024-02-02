<template>
  <a-form-model ref="form" :model="form" :rules="rules" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
    <a-form-model-item :label="$t('oneterm.protocol')">
      <div class="protocol-box" v-for="(pro, index) in protocols" :key="pro.id">
        <a-input-group compact>
          <a-select v-model="pro.value" style="width: 100px">
            <a-select-option value="ssh">
              ssh
            </a-select-option>
          </a-select>
          <a-input :placeholder="$t('oneterm.assetList.protocolPlaceholder')" v-model="pro.label" style="width: calc(100% - 100px)" />
        </a-input-group>
        <a-space>
          <a @click="addPro"><a-icon type="plus-circle"/></a>
          <a
            v-if="protocols && protocols.length > 1"
            style="color:red"
            @click="deletePro(index)"
          ><ops-icon
            type="icon-xianxing-delete"
          /></a>
        </a-space>
      </div>
    </a-form-model-item>
    <a-form-model-item
      :label="$t('oneterm.gateway')"
      prop="gateway_id"
      :style="{ display: 'flex', alignItems: 'center' }"
    >
      <treeselect
        class="custom-treeselect custom-treeselect-bgcAndBorder"
        :style="{
          '--custom-height': '32px',
          lineHeight: '32px',
          '--custom-bg-color': '#fff',
          '--custom-border': '1px solid #d9d9d9',
        }"
        v-model="form.gateway_id"
        :placeholder="`${$t(`placeholder2`)}`"
        :multiple="false"
        :clearable="true"
        searchable
        :options="gatewayList"
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
  </a-form-model>
</template>

<script>
import { v4 as uuidv4 } from 'uuid'
import { getGatewayList } from '../../../api/gateway'
export default {
  name: 'Protocol',
  data() {
    return {
      form: {
        gateway_id: undefined,
      },
      rules: {},
      protocols: [{ id: uuidv4(), value: 'ssh', label: '22' }],
      gatewayList: [],
    }
  },
  mounted() {
    getGatewayList({ page_index: 1 }).then((res) => {
      this.gatewayList = res?.data?.list || []
    })
  },
  methods: {
    addPro() {
      this.protocols.push({ id: uuidv4(), value: 'ssh', label: '22' })
    },
    deletePro(index) {
      this.protocols.splice(index, 1)
    },
    getValues() {
      const { gateway_id } = this.form
      const _protocols = this.protocols.map((pro) => `${pro.value}:${pro.label}`)
      return { gateway_id, protocols: _protocols }
    },
    setValues({ gateway_id = undefined, protocols = [] }) {
      this.form = { gateway_id: gateway_id || undefined }
      this.protocols = protocols.length
        ? protocols.map((p) => ({
            id: uuidv4(),
            value: p.split(':')[0],
            label: p.split(':')[1],
          }))
        : [{ id: uuidv4(), value: 'ssh', label: '22' }]
    },
  },
}
</script>

<style lang="less" scoped>
.protocol-box {
  position: relative;
  .ant-space {
    position: absolute;
    right: -45px;
  }
}
</style>
