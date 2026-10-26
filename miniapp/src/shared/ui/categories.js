export const categories = [
  { code: 'CRACK', label: '裂缝' },
  { code: 'DISCOLOURATION', label: '变色' },
  { code: 'EFFLORESCENCE', label: '泛碱' },
  { code: 'SURFACE_DISSOLUTION', label: '表面溶蚀' },
  { code: 'MASONRY_LOSS', label: '砌体缺失' },
  { code: 'FUNGUS', label: '真菌附着' },
  { code: 'MOSS', label: '苔藓' },
  { code: 'PLANT', label: '植物附着' },
  { code: 'CORROSION', label: '腐蚀' },
  { code: 'ARTISTIC', label: '人为痕迹' },
  { code: 'UNREACHABLE', label: '现场不可达' },
]
export function categoryLabel(code) { return categories.find(item => item.code === code)?.label || code || '选择病害类别' }
