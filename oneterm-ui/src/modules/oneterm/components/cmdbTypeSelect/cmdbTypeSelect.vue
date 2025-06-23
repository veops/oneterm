<template>
  <treeselect
    :disabled="disabled"
    ref="cmdb_type_select"
    :disable-branch-nodes="true"
    class="custom-treeselect custom-treeselect-white"
    :style="{
      '--custom-height': '30px',
      lineHeight: '30px'
    }"
    v-model="currenCiType"
    :multiple="false"
    :clearable="true"
    searchable
    :options="ciTypeGroup"
    value-consists-of="LEAF_PRIORITY"
    :placeholder="`${$t(`placeholder2`)}`"
    :load-options="loadOptions"
    @select="
      (node, instanceId) => {
        $emit('select', node, instanceId)
      }
    "
    @deselect="
      (node, instanceId) => {
        $emit('deselect', node, instanceId)
      }
    "
    :normalizer="
      (node) => {
        return {
          id: node.id || -1,
          label: node.alias || node.name || $t('other'),
          title: node.alias || node.name || $t('other'),
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
    <div slot="value-label" slot-scope="{ node }">{{ getTreeSelectLabel(node) }}</div>
  </treeselect>
</template>

<script>
import _ from 'lodash'
import { getCITypeGroups, getCITypeAttributesById } from '../../api/otherModules'
import { getTreeSelectLabel } from '../../utils'
export default {
  name: 'CMDBTypeSelect',
  model: {
    prop: 'value',
    event: 'change',
  },
  props: {
    value: {
      type: [String, Number],
      default: null,
    },
    selectType: {
      type: String,
      default: 'attributes',
    },
    attrIdkey: {
      type: String,
      default: 'id',
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      ciTypeGroup: [],
      childrenOptions: [],
    }
  },
  computed: {
    currenCiType: {
      get() {
        return this.value
      },
      set(val) {
        this.$emit('change', val)
        return val
      },
    },
  },
  async mounted() {
    if (this.value) {
      const typeId = this.value.split('-')[0]
      await getCITypeAttributesById(this.value.split('-')[0]).then((res) => {
        this.childrenOptions = res.attributes.map((item) => ({ ...item, id: `${typeId}-${item[this.attrIdkey]}` }))
      })
    }
    this.getCITypeGroups()
  },
  methods: {
    getTreeSelectLabel,
    getCITypeGroups() {
      getCITypeGroups({ need_other: true }).then((res) => {
        this.ciTypeGroup = res
          .filter((item) => item.ci_types && item.ci_types.length)
          .map((item) => {
            item.id = `type_${item.id || -1}`
            item.children = item.ci_types.map((type) => {
              const obj = { ...type }
              if (this.selectType === 'attributes') {
                obj.children = this.value && type.id === Number(this.value.split('-')[0]) ? this.childrenOptions : null
              }
              return obj
            })
            return { ..._.cloneDeep(item) }
          })
      })
    },
    loadOptions({ action, parentNode, callback }) {
      getCITypeAttributesById(parentNode.id).then((res) => {
        parentNode.children = res.attributes.map((item) => ({
          ...item,
          id: `${parentNode.id}-${item[this.attrIdkey]}`,
        }))
        callback()
      })
    },
  },
}
</script>

<style></style>
