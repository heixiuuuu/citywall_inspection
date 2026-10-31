<template>
  <div class="overview" :aria-busy="loading">
    <div class="welcome">
      <div><p class="eyebrow">古城墙预防性保护 · 巡检工作台</p><h1>掌握现场，跟进每一处变化</h1><p class="muted">从任务派发到问题闭环，集中查看巡检进展与待办。</p></div>
      <el-button :loading="loading" @click="load">刷新概况</el-button>
    </div>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-skeleton v-if="!summary && loading" :rows="10" animated />
    <template v-if="summary">
      <div class="metrics">
        <router-link v-for="metric in metrics" :key="metric.label" :to="metric.to" class="metric">
          <span>{{ metric.label }}</span><strong>{{ metric.value.toLocaleString() }}</strong><small>{{ metric.hint }} <span aria-hidden="true">↗</span></small>
        </router-link>
      </div>
      <div class="columns">
        <section class="panel todo">
          <div class="section-heading"><div><p class="eyebrow">下一步行动</p><h2>待办工作</h2></div><span class="muted">按业务环节跟进</span></div>
          <router-link v-for="item in todos" :key="item.label" :to="item.to" class="todo-row">
            <div><strong>{{ item.label }}</strong><p>{{ item.hint }}</p></div><span class="count" :class="{ attention: item.value > 0 }">{{ item.value }} <span aria-hidden="true">→</span></span>
          </router-link>
        </section>
        <section class="panel">
          <div class="section-heading"><div><p class="eyebrow">人工确认</p><h2>病害与植物分布</h2></div></div>
          <p class="muted explanation">仅统计审核通过记录最后一次提交的人工结论项，排除误检；同一病害在不同记录中可重复出现。</p>
          <el-empty v-if="!summary.categories.length" description="尚无审核通过的病害结论" :image-size="72"><el-button text type="primary" @click="$router.push('/reviews')">前往审核管理</el-button></el-empty>
          <div v-for="item in summary.categories" :key="item.code" class="distribution">
            <div><span>{{ categoryLabel(item.code) }}</span><strong>{{ item.total }} 项</strong></div>
            <div class="track"><span :style="{ width: `${item.total / maxCategory * 100}%` }" /></div>
          </div>
        </section>
      </div>
      <div class="columns secondary">
        <section class="panel">
          <div class="section-heading"><div><p class="eyebrow">业务进展</p><h2>任务状态</h2></div><router-link to="/tasks">查看任务 →</router-link></div>
          <div class="status-grid"><router-link v-for="state in taskStates" :key="state.code" :to="{ path: '/tasks', query: { status: state.code } }"><span>{{ state.label }}</span><strong>{{ count(summary.tasks, state.code) }}</strong></router-link></div>
        </section>
        <section class="panel quality">
          <div class="section-heading"><div><p class="eyebrow">数据完整性</p><h2>需要留意的记录</h2></div></div>
          <dl><div><dt>已提交记录缺少有效定位</dt><dd>{{ summary.missing_location }} 条</dd></div><div><dt>已提交记录定位精度超过 30m</dt><dd>{{ summary.low_accuracy }} 条</dd></div><div><dt>当前关联照片最新识别失败</dt><dd>{{ summary.failed_recognition }} 张</dd></div></dl>
          <p class="muted explanation">定位提示用于辅助复核；识别失败可继续人工填写。这里展示当前记录与照片的状态。</p>
          <router-link to="/records">查看现场记录 →</router-link>
        </section>
      </div>
      <footer><span>有效巡检点位 {{ summary.active_points }} 个 · 统计范围为全库当前业务数据</span><time :datetime="summary.generated_at">更新于 {{ formatDateTime(summary.generated_at) }}</time></footer>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { fetchOverview, type Overview, type OverviewCount } from '@/shared/api/overview'
import { categoryLabel } from '@/shared/ui/labels'
import { formatDateTime } from '@/shared/ui/time'

const summary = ref<Overview | null>(null)
const loading = ref(false)
const error = ref('')
const count = (items: OverviewCount[], code: string) => items.find(item => item.code === code)?.total ?? 0
const sum = (items: OverviewCount[]) => items.reduce((total, item) => total + item.total, 0)
const maxCategory = computed(() => Math.max(1, ...summary.value?.categories.map(item => item.total) ?? []))
const metrics = computed(() => {
  const s = summary.value!
  return [
    { label: '巡检任务', value: sum(s.tasks), hint: `已完成 ${count(s.tasks, 'COMPLETED')} 项`, to: '/tasks' },
    { label: '现场记录', value: sum(s.records), hint: `审核通过 ${count(s.records, 'ACCEPTED')} 条`, to: '/records' },
    { label: '未关闭问题', value: sum(s.issues) - count(s.issues, 'CLOSED'), hint: `需尽快核实 ${s.urgent_issues} 项`, to: '/issues' },
    { label: '超期未完成任务', value: s.overdue_tasks, hint: '已派发且超过截止时间', to: '/tasks?overdue=1' },
  ]
})
const todos = computed(() => {
  const s = summary.value!
  return [
    { label: '现场记录待审核', hint: '核对照片、定位与人工结论', value: count(s.records, 'SUBMITTED'), to: '/reviews' },
    { label: '通过记录待受理', hint: '建立问题、关联台账或说明不立项原因', value: s.pending_admission, to: '/records?status=ACCEPTED&pending_admission=1' },
    { label: '问题待分配', hint: '明确责任人，推进后续处置', value: count(s.issues, 'UNASSIGNED'), to: '/issues?status=UNASSIGNED' },
    { label: '处置结果待核验', hint: '核对处理结果并完成关闭或退回', value: count(s.issues, 'PENDING_VERIFICATION'), to: '/issues?status=PENDING_VERIFICATION' },
    { label: '任务待完成确认', hint: '确认点位记录与审核结果', value: count(s.tasks, 'PENDING_REVIEW'), to: '/tasks?status=PENDING_REVIEW' },
  ]
})
const taskStates = [
  { code: 'DRAFT', label: '草稿' }, { code: 'ASSIGNED', label: '待执行' },
  { code: 'IN_PROGRESS', label: '进行中' }, { code: 'RETURNED', label: '退回补正' },
  { code: 'PENDING_REVIEW', label: '待审核' }, { code: 'COMPLETED', label: '已完成' }, { code: 'CANCELLED', label: '已取消' },
]
async function load() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try { summary.value = await fetchOverview() }
  catch { error.value = '概况暂时无法加载，请检查服务后重试。' }
  finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped>
.overview { display: grid; gap: var(--space-section); max-width: 1480px; margin: auto; }
.welcome, .section-heading, footer { display: flex; align-items: center; justify-content: space-between; gap: var(--space-content); }
h1, h2, p { margin: 0; }
h1 { font-size: 26px; font-weight: 600; letter-spacing: -.02em; color: var(--text-strong); margin: 6px 0; }
h2 { font-size: 18px; font-weight: 600; }
.eyebrow { font-size: 12px; color: var(--brand); letter-spacing: .06em; }
.muted, footer, small { color: var(--text-muted); font-size: 13px; }
.metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: var(--space-content); }
a { color: var(--brand); text-decoration: none; }
.metric, .panel { background: var(--surface); border-radius: var(--radius-panel); padding: var(--space-section); }
.metric { display: grid; gap: 12px; color: var(--text-main); transition: background .16s; }
.metric:hover, .status-grid a:hover, .todo-row:hover { background: var(--surface-hover); }
.metric > strong { font-size: 34px; line-height: 1.2; font-weight: 600; color: var(--brand); }
.metric small { display: flex; justify-content: space-between; }
.columns { display: grid; grid-template-columns: 1.1fr 1fr; gap: var(--space-section); }
.section-heading { margin-bottom: var(--space-content); }
.todo-row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-content); padding: 14px 10px; margin: 0 -10px; border-radius: var(--radius-control); color: var(--text-main); }
.todo-row strong { font-size: 14px; font-weight: 500; }
.todo-row p { font-size: 12px; color: var(--text-muted); margin-top: 4px; }
.count { white-space: nowrap; font-size: 20px; color: var(--text-muted); }
.count span { font-size: 14px; margin-left: 10px; }
.attention { color: var(--brand); font-weight: 600; }
.explanation { line-height: 1.7; margin-bottom: var(--space-content); }
.distribution { margin-top: var(--space-content); }
.distribution > div:first-child { display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px; }
.distribution strong { font-weight: 500; }
.track { height: 7px; border-radius: var(--radius-control); background: var(--surface-sunken); overflow: hidden; }
.track span { display: block; height: 100%; background: var(--brand-soft); border-radius: inherit; }
.status-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.status-grid a { display: grid; gap: 8px; background: var(--surface-subtle); padding: 14px; border-radius: var(--radius-control); font-size: 13px; color: var(--text-muted); }
.status-grid strong { font-size: 23px; color: var(--text-strong); font-weight: 500; }
dl { margin: 0 0 var(--space-content); }
dl > div { display: flex; justify-content: space-between; gap: var(--space-content); padding: 10px 0; font-size: 14px; }
dd { margin: 0; white-space: nowrap; color: var(--brand); }
footer { font-size: 12px; padding-bottom: 8px; flex-wrap: wrap; }
@media (max-width: 1100px) { .metrics { grid-template-columns: repeat(2, 1fr); } .columns { grid-template-columns: 1fr; } }
@media (max-width: 600px) { h1 { font-size: 21px; } .welcome { align-items: flex-start; flex-direction: column; } .metric, .panel { padding: var(--space-content); } .status-grid { grid-template-columns: repeat(2, 1fr); } }
</style>
