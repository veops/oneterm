<template>
  <div>
    <a-space :style="{ display: 'flex', marginBottom: '10px' }" v-for="(item, index) in ruleList" :key="item.id">
      <div :style="{ width: '70px', height: '24px', position: 'relative' }">
        <treeselect
          v-if="index"
          class="custom-treeselect"
          :style="{ width: '70px', '--custom-height': '24px', position: 'absolute', top: '-17px', left: 0 }"
          v-model="item.type"
          :multiple="false"
          :clearable="false"
          searchable
          :options="ruleTypeList"
          :normalizer="
            (node) => {
              return {
                id: node.value,
                label: node.label,
                children: node.children,
              }
            }
          "
          :disabled="disabled"
        >
        </treeselect>
      </div>
      <treeselect
        class="custom-treeselect"
        :style="{ width: '130px', '--custom-height': '24px' }"
        v-model="item.property"
        :multiple="false"
        :clearable="false"
        searchable
        :options="canSearchPreferenceAttrList"
        :normalizer="
          (node) => {
            return {
              id: node.name,
              label: node.alias || node.name,
              children: node.children,
            }
          }
        "
        appendToBody
        :zIndex="1050"
        :disabled="disabled"
      >
        <div
          :title="node.label"
          slot="option-label"
          slot-scope="{ node }"
          :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
        >
          <ValueTypeMapIcon :attr="node.raw" />
          {{ node.label }}
        </div>
        <div
          :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
          slot="value-label"
          slot-scope="{ node }"
        >
          <ValueTypeMapIcon :attr="node.raw" /> {{ node.label }}
        </div>
      </treeselect>
      <treeselect
        class="custom-treeselect"
        :style="{ width: '100px', '--custom-height': '24px' }"
        v-model="item.exp"
        :multiple="false"
        :clearable="false"
        searchable
        :options="[...getExpListByProperty(item.property), ...advancedExpList]"
        :normalizer="
          (node) => {
            return {
              id: node.value,
              label: node.label,
              children: node.children,
            }
          }
        "
        @select="(value) => handleChangeExp(value, item, index)"
        appendToBody
        :zIndex="1050"
        :disabled="disabled"
      >
        <div
          slot="option-label"
          slot-scope="{ node }"
          :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
        >
          <a-tooltip :title="node.label">
            {{ node.label }}
          </a-tooltip>
        </div>
        <div
          :style="{ width: '100%', whiteSpace: 'nowrap', textOverflow: 'ellipsis', overflow: 'hidden' }"
          slot="value-label"
          slot-scope="{ node }"
        >
          <a-tooltip :title="node.label">
            {{ node.label }}
          </a-tooltip>
        </div>
      </treeselect>
      <CIReferenceAttr
        v-if="getAttr(item.property).is_reference && (item.exp === 'is' || item.exp === '~is')"
        :style="{ width: '175px' }"
        class="select-filter-component ops-select-bg"
        :referenceTypeId="getAttr(item.property).reference_type_id"
        :disabled="disabled"
        v-model="item.value"
      />
      <a-select
        v-else-if="getAttr(item.property).is_bool && (item.exp === 'is' || item.exp === '~is')"
        v-model="item.value"
        class="select-filter-component ops-select-bg"
        :style="{ width: '175px' }"
        :disabled="disabled"
        :placeholder="$t('placeholder2')"
      >
        <a-select-option key="1">
          true
        </a-select-option>
        <a-select-option key="0">
          false
        </a-select-option>
      </a-select>
      <treeselect
        class="custom-treeselect"
        :style="{ width: '175px', '--custom-height': '24px' }"
        v-model="item.value"
        :multiple="false"
        :clearable="false"
        searchable
        v-else-if="isChoiceByProperty(item.property) && (item.exp === 'is' || item.exp === '~is')"
        :options="getChoiceValueByProperty(item.property)"
        :placeholder="$t('placeholder2')"
        :normalizer="
          (node) => {
            return {
              id: String(node[0] || ''),
              label: node[1] ? node[1].label || node[0] : node[0],
              children: node.children && node.children.length ? node.children : undefined,
            }
          }
        "
        appendToBody
        :zIndex="1050"
        :disabled="disabled"
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
      <a-input-group
        size="small"
        compact
        v-else-if="item.exp === 'range' || item.exp === '~range'"
        :style="{ width: '175px' }"
      >
        <a-input
          class="ops-input"
          size="small"
          v-model="item.min"
          :style="{ width: '78px' }"
          :placeholder="$t('min')"
          :disabled="disabled"
        />
        ~
        <a-input
          class="ops-input"
          size="small"
          v-model="item.max"
          :style="{ width: '78px' }"
          :placeholder="$t('max')"
          :disabled="disabled"
        />
      </a-input-group>
      <a-input-group size="small" compact v-else-if="item.exp === 'compare'" :style="{ width: '175px' }">
        <treeselect
          class="custom-treeselect"
          :style="{ width: '60px', '--custom-height': '24px' }"
          v-model="item.compareType"
          :multiple="false"
          :clearable="false"
          searchable
          :options="compareTypeList"
          :normalizer="
            (node) => {
              return {
                id: node.value,
                label: node.label,
                children: node.children,
              }
            }
          "
          appendToBody
          :zIndex="1050"
          :disabled="disabled"
        >
        </treeselect>
        <a-input class="ops-input" v-model="item.value" size="small" style="width: 113px" />
      </a-input-group>
      <a-input
        v-else-if="item.exp !== 'value' && item.exp !== '~value'"
        size="small"
        v-model="item.value"
        :placeholder="item.exp === 'in' || item.exp === '~in' ? $t('cmdbFilterComp.split', { separator: ';' }) : ''"
        class="ops-input"
        :style="{ width: '175px' }"
        :disabled="disabled"
      ></a-input>
      <div v-else :style="{ width: '175px' }"></div>
      <template v-if="!disabled">
        <a-tooltip :title="$t('copy')">
          <a class="operation" @click="handleCopyRule(item)"><ops-icon type="veops-copy"/></a>
        </a-tooltip>
        <a-tooltip :title="$t('delete')">
          <a class="operation" @click="handleDeleteRule(item)"><ops-icon type="icon-xianxing-delete"/></a>
        </a-tooltip>
        <a-tooltip :title="$t('cmdbFilterComp.addHere')" v-if="needAddHere">
          <a class="operation" @click="handleAddRuleAt(item)"><a-icon type="plus-circle"/></a>
        </a-tooltip>
      </template>
    </a-space>
    <div class="table-filter-add" v-if="!disabled">
      <a @click="handleAddRule">+ {{ $t('new') }}</a>
    </div>
  </div>
</template>

<script>
import _ from 'lodash'
import { v4 as uuidv4 } from 'uuid'
import { ruleTypeList, expList, advancedExpList, compareTypeList } from './constants'
import ValueTypeMapIcon from '../CMDBValueTypeMapIcon'
import CIReferenceAttr from '../ciReferenceAttr/index.vue'

export default {
  name: 'Expression',
  components: { ValueTypeMapIcon, CIReferenceAttr },
  model: {
    prop: 'value',
    event: 'change',
  },
  props: {
    value: {
      type: Array,
      default: () => [],
    },
    canSearchPreferenceAttrList: {
      type: Array,
      required: true,
      default: () => [],
    },
    needAddHere: {
      type: Boolean,
      default: false,
    },
    disabled: {
      type: Boolean,
      default: false,
    },
  },
  data() {
    return {
      compareTypeList,
    }
  },
  computed: {
    ruleList: {
      get() {
        return this.value
      },
      set(val) {
        this.$emit('change', val)
        return val
      },
    },
    ruleTypeList() {
      return ruleTypeList()
    },
    expList() {
      return expList()
    },
    advancedExpList() {
      return advancedExpList()
    },
  },
  methods: {
    getExpListByProperty(property) {
      if (property) {
        const _find = this.canSearchPreferenceAttrList.find((item) => item.name === property)
        if (_find && (['0', '1', '3', '4', '5'].includes(_find.value_type) || _find.is_reference || _find.is_bool)) {
          return [
            { value: 'is', label: this.$t('cmdbFilterComp.is') },
            { value: '~is', label: this.$t('cmdbFilterComp.~is') },
            { value: '~value', label: this.$t('cmdbFilterComp.~value') },
            { value: 'value', label: this.$t('cmdbFilterComp.value') },
          ]
        }
        return this.expList
      }
      return this.expList
    },
    isChoiceByProperty(property) {
      const _find = this.canSearchPreferenceAttrList.find((item) => item.name === property)
      if (_find) {
        return _find.is_choice
      }
      return false
    },
    handleAddRule() {
      this.ruleList.push({
        id: uuidv4(),
        type: 'and',
        property: this.canSearchPreferenceAttrList[0]?.name,
        exp: 'is',
        value: null,
      })
      this.$emit('change', this.ruleList)
    },
    handleCopyRule(item) {
      this.ruleList.push({ ...item, id: uuidv4() })
      this.$emit('change', this.ruleList)
    },
    handleDeleteRule(item) {
      const idx = this.ruleList.findIndex((r) => r.id === item.id)
      if (idx > -1) {
        this.ruleList.splice(idx, 1)
      }
      this.$emit('change', this.ruleList)
    },
    handleAddRuleAt(item) {
      const idx = this.ruleList.findIndex((r) => r.id === item.id)
      if (idx > -1) {
        this.ruleList.splice(idx, 0, {
          id: uuidv4(),
          type: 'and',
          property: this.canSearchPreferenceAttrList[0]?.name,
          exp: 'is',
          value: null,
        })
      }
      this.$emit('change', this.ruleList)
    },
    getChoiceValueByProperty(property) {
      const _find = this.canSearchPreferenceAttrList.find((item) => item.name === property)
      if (_find) {
        return _find.choice_value
      }
      return []
    },
    getAttr(property) {
      return this.canSearchPreferenceAttrList.find((item) => item.name === property) || {}
    },
    handleChangeExp({ value }, item, index) {
      const _ruleList = _.cloneDeep(this.ruleList)
      if (value === 'range') {
        _ruleList[index] = {
          ..._ruleList[index],
          min: '',
          max: '',
          exp: value,
        }
      } else if (value === 'compare') {
        _ruleList[index] = {
          ..._ruleList[index],
          compareType: '1',
          exp: value,
        }
      } else {
        _ruleList[index] = {
          ..._ruleList[index],
          exp: value,
        }
      }
      this.ruleList = _ruleList
      this.$emit('change', this.ruleList)
    },
  },
}
</script>

<style lang="less" scoped>
.select-filter-component {
  height: 24px;

  /deep/ .ant-select-selection {
    height: 24px;
    line-height: 24px;
    border: none;

    .ant-select-selection__rendered {
      height: 24px;
      line-height: 24px;
    }
  }
}
</style>
