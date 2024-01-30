<template>
  <SessionTable :status="2" />
</template>

<script>
import SessionTable from './sesstionTable.vue'
export default {
  name: 'SessionHistory',
  components: { SessionTable },
}
</script>

<style></style>
