<template>
    <el-dialog v-model="visible" title="受理问题" width="560" :close-on-click-modal="false" :close-on-press-escape="!admitting" :show-close="!admitting">
      <el-form label-width="100px">
        <el-form-item label="受理方式" required>
          <el-radio-group v-model="admitMode">
            <el-radio value="NEW">新建问题</el-radio>
            <el-radio value="LINK">关联已有问题</el-radio>
            <el-radio value="NONE">不立项</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="admitMode === 'NEW'">
          <el-form-item label="标题" required>
            <el-input v-model="admitForm.title" placeholder="如：南门段墙体裂缝" />
          </el-form-item>
          <el-form-item label="优先级" required>
            <el-select v-model="admitForm.priority_code" style="width: 160px">
              <el-option label="关注" value="WATCH" />
              <el-option label="需处理" value="PROCESS" />
              <el-option label="尽快核实" value="URGENT" />
            </el-select>
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="admitForm.description" type="textarea" :rows="2" placeholder="问题描述与初步判断" />
          </el-form-item>
          <el-form-item label="优先级依据">
            <el-input v-model="admitForm.priority_reason" placeholder="如：裂缝延伸至女儿墙，建议尽快核实" />
          </el-form-item>
        </template>
        <template v-else-if="admitMode === 'LINK'">
          <el-form-item label="关联问题" required>
            <el-select v-model="admitForm.link_issue_ids" multiple filterable style="width: 100%" placeholder="选择开放中的问题">
              <el-option v-for="i in openIssues" :key="i.id" :label="`#${i.id} ${i.title}`" :value="i.id" />
            </el-select>
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="不立项原因" required>
            <el-input v-model="admitForm.no_issue_reason" type="textarea" :rows="2" placeholder="如：经核实现场无明显病害，判定为误检" />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button :disabled="admitting" @click="visible = false">取消</el-button>
        <el-button type="primary" :loading="admitting" @click="submitAdmit">提交受理</el-button>
      </template>
    </el-dialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchIssues, admitObservation, type Issue } from '@/shared/api/issue'
const props = defineProps<{ modelValue: boolean; observationId: number }>()
const emit = defineEmits(['update:modelValue', 'admitted'])
const visible = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) })
const admitting = ref(false)
const admitMode = ref<'NEW' | 'LINK' | 'NONE'>('NEW')
const openIssues = ref<Issue[]>([])
const admitForm = ref<{
  title: string
  priority_code: 'WATCH' | 'PROCESS' | 'URGENT'
  description: string
  priority_reason: string
  link_issue_ids: number[]
  no_issue_reason: string
}>({ title: '', priority_code: 'WATCH', description: '', priority_reason: '', link_issue_ids: [], no_issue_reason: '' })

watch(() => props.modelValue, value => { if (value) resetForm() })
async function resetForm() {
  admitMode.value = 'NEW'
  admitForm.value = { title: '', priority_code: 'WATCH', description: '', priority_reason: '', link_issue_ids: [], no_issue_reason: '' }
  openIssues.value = []
  try {
    let page = 1, loaded = 0
    while (true) {
      const res = await fetchIssues({ page, page_size: 100 })
      openIssues.value.push(...res.items.filter(item => item.status !== 'CLOSED'))
      loaded += res.items.length
      if (!res.items.length || loaded >= res.total) break
      page++
    }
  } catch { ElMessage.error('关联问题加载失败，请关闭后重试') }
}

async function submitAdmit() {
  if (admitMode.value === 'NEW' && !admitForm.value.title.trim()) {
    ElMessage.warning('请填写问题标题')
    return
  }
  if (admitMode.value === 'LINK' && !admitForm.value.link_issue_ids.length) {
    ElMessage.warning('请选择要关联的问题')
    return
  }
  if (admitMode.value === 'NONE' && !admitForm.value.no_issue_reason.trim()) {
    ElMessage.warning('请填写不立项原因')
    return
  }
  admitting.value = true
  try {
    const payload =
      admitMode.value === 'NEW'
        ? { new_issues: [{ title: admitForm.value.title, priority_code: admitForm.value.priority_code, description: admitForm.value.description || undefined, priority_reason: admitForm.value.priority_reason || undefined }] }
        : admitMode.value === 'LINK'
          ? { link_issue_ids: admitForm.value.link_issue_ids }
          : { no_issue_reason: admitForm.value.no_issue_reason }
    const res = await admitObservation(props.observationId, payload)
    const a = res.admit
    if (a.already_admitted) ElMessage.info(a.no_issue_reason ? `该记录已不立项：${a.no_issue_reason}` : '该记录已受理（返回已有关联）')
    else if (a.no_issue_reason) ElMessage.success('已记录不立项原因')
    else ElMessage.success(`受理完成：新建 ${a.created_issue_ids?.length ?? 0} 个，关联 ${a.linked_issue_ids?.length ?? 0} 个`)
    visible.value = false
    emit('admitted')
  } catch (e: any) {
    ElMessage.error(e?.message || '受理失败')
  } finally {
    admitting.value = false
  }
}
</script>
