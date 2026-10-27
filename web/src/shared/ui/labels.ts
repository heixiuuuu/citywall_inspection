const categories: Record<string, string> = {
  CRACK: '裂缝', DISCOLOURATION: '变色', EFFLORESCENCE: '泛碱',
  SURFACE_DISSOLUTION: '表面溶蚀', MASONRY_LOSS: '砌体缺失', FUNGUS: '真菌附着',
  MOSS: '苔藓', PLANT: '植物附着', CORROSION: '腐蚀', ARTISTIC: '人为痕迹', UNREACHABLE: '现场不可达',
}
const outcomes: Record<string, string> = {
  NORMAL: '正常', SUSPECTED: '疑似病害', UNDETERMINED: '不确定', UNREACHABLE: '现场不可达',
}
const statuses: Record<string, string> = {
  DRAFT: '草稿', SUBMITTED: '待审核', RETURNED: '已退回', ACCEPTED: '已通过', REJECTED: '已驳回',
}
const reviews: Record<string, string> = { ACCEPT: '通过', RETURN: '退回补正', REJECT: '驳回' }
const decisions: Record<string, string> = { CONFIRM: '确认', MODIFY: '修正', REJECT: '误检', ADD: '补录' }
const attempts: Record<string, string> = { PENDING: '等待识别', RUNNING: '识别中', SUCCEEDED: '识别成功', FAILED: '识别失败' }

function label(code: string | null | undefined, names: Record<string, string>) {
  return code ? names[code] ?? code : '—'
}
export const categoryLabel = (code: string | null | undefined) => label(code, categories)
export const outcomeLabel = (code: string | null | undefined) => label(code, outcomes)
export const observationStatusLabel = (code: string | null | undefined) => label(code, statuses)
export const reviewLabel = (code: string | null | undefined) => label(code, reviews)
export const decisionLabel = (code: string | null | undefined) => label(code, decisions)
export const attemptLabel = (code: string | null | undefined) => label(code, attempts)
