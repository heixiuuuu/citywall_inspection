<template>
  <ObservationEditor v-if="query" :query="query" source="TASK" />
</template>
<script setup>
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import ObservationEditor from '../../shared/ui/ObservationEditor.vue'
const query = ref(null)
onLoad(value => { query.value = value })
</script>
