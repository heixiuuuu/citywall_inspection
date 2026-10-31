<template>
    <el-dialog v-model="visible" title="受理问题" width="560" :close-on-click-modal="false" :close-on-press-escape="!admitting" :show-close="!admitting">
      <el-alert v-if="sourceLocation" :title="`来源：${sourceLocation}`" type="info" :closable="false" class="source-notice" />
      <el-form label-width="100px" :disabled="admitting || loadingContext">
        <el-form-item label="受理方式" required>
          <el-radio-group v-model="admitMode">
            <el-radio value="NEW">新建问题</el-radio>
            <el-radio value="LINK">关联已有问题</el-radio>
            <el-radio value="NONE">不立项</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="admitMode === 'NEW'">
          <el-form-item label="标题" required>
            <el-input v-model="admitForm.title" maxlength="128" placeholder="如：南门段墙体裂缝" />
          </el-form-item>
          <el-form-item label="优先级" required>
            <el-radio-group v-model="admitForm.priority_code"><el-radio value="WATCH">关注</el-radio><el-radio value="PROCESS">需处理</el-radio><el-radio value="URGENT">尽快核实</el-radio></el-radio-group>
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
            <IssueSelect :key="observationId" v-model="admitForm.link_issue_ids" />
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
        <el-button type="primary" :loading="admitting" :disabled="loadingContext" @click="submitAdmit">提交受理</el-button>
      </template>
    </el-dialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { admitObservation } from '@/shared/api/issue'
import { fetchEvidence } from '@/shared/api/observation'
import { categoryLabel } from '@/shared/ui/labels'
import IssueSelect from './IssueSelect.vue'
const props = defineProps<{ modelValue: boolean; observationId: number }>()
const emit = defineEmits(['update:modelValue', 'admitted'])
const visible = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) })
const admitting = ref(false)
const admitMode = ref<'NEW' | 'LINK' | 'NONE'>('NEW')
const sourceLocation = ref('')
const loadingContext = ref(false)
let generation = 0
const admitForm = ref<{
  title: string
  priority_code: 'WATCH' | 'PROCESS' | 'URGENT'
  description: string
  priority_reason: string
  link_issue_ids: number[]
  no_issue_reason: string
}>({ title: '', priority_code: 'WATCH', description: '', priority_reason: '', link_issue_ids: [], no_issue_reason: '' })

watch(() => [props.modelValue, props.observationId], () => { if (props.modelValue) void resetForm(); else generation++ })
async function resetForm() {
  const current = ++generation
  admitMode.value = 'NEW'
  admitForm.value = { title: '', priority_code: 'WATCH', description: '', priority_reason: '', link_issue_ids: [], no_issue_reason: '' }
  sourceLocation.value = ''
  loadingContext.value = true
  try {
    const evidence = await fetchEvidence(props.observationId)
    if (current !== generation) return
    sourceLocation.value = evidence.observation.location_text || `现场记录 #${props.observationId}`
    const submission = [...evidence.submissions].sort((a, b) => b.submission_no - a.submission_no)[0]
    const conclusion = evidence.conclusions.find(item => item.id === submission?.conclusion_revision_id)
    const classes = [...new Set(evidence.items.filter(item => item.conclusion_revision_id === conclusion?.id && ['CONFIRM', 'MODIFY', 'ADD'].includes(item.decision)).map(item => categoryLabel(item.final_class_code)))]
    admitForm.value.title = `${sourceLocation.value}${classes.length ? ` · ${classes.join('、')}` : ''}`.slice(0, 128)
    admitForm.value.description = conclusion?.description || ''
    if (conclusion?.priority_code && ['WATCH', 'PROCESS', 'URGENT'].includes(conclusion.priority_code)) admitForm.value.priority_code = conclusion.priority_code as 'WATCH' | 'PROCESS' | 'URGENT'
  } catch { if (current === generation) ElMessage.warning('来源摘要暂时无法加载，可手动填写受理信息') }
  finally { if (current === generation) loadingContext.value = false }
}

async function submitAdmit() {
  if (admitting.value || loadingContext.value) return
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
        ? { new_issues: [{ title: admitForm.value.title.trim(), priority_code: admitForm.value.priority_code, description: admitForm.value.description.trim() || undefined, priority_reason: admitForm.value.priority_reason.trim() || undefined }] }
        : admitMode.value === 'LINK'
          ? { link_issue_ids: admitForm.value.link_issue_ids }
          : { no_issue_reason: admitForm.value.no_issue_reason.trim() }
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
<style scoped>
.source-notice { margin-bottom: var(--space-content); }
</style>
