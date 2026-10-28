package report

var reportLabels = map[string]string{
	"DRAFT": "草稿", "ASSIGNED": "已派发", "IN_PROGRESS": "进行中", "PENDING_REVIEW": "待审核", "COMPLETED": "已完成", "CANCELLED": "已取消",
	"SUBMITTED": "待审核", "RETURNED": "已退回", "ACCEPTED": "已通过", "REJECTED": "已驳回",
	"NORMAL": "正常", "SUSPECTED": "疑似病害", "UNDETERMINED": "不确定", "UNREACHABLE": "现场不可达",
	"TASK": "任务巡检", "VOLUNTARY": "主动上报",
	"PENDING": "等待识别", "RUNNING": "识别中", "SUCCEEDED": "识别成功", "FAILED": "识别失败",
	"CRACK": "裂缝", "DISCOLOURATION": "变色", "EFFLORESCENCE": "泛碱", "SURFACE_DISSOLUTION": "表面溶蚀",
	"MASONRY_LOSS": "砌体缺失", "FUNGUS": "真菌附着", "MOSS": "苔藓", "PLANT": "植物附着", "CORROSION": "腐蚀", "ARTISTIC": "人为痕迹",
	"SURFACE_SPALLING": "表面剥落",
	"CONFIRM":          "确认", "MODIFY": "修正", "REJECT": "误检", "ADD": "补录",
	"WATCH": "关注", "PROCESS": "需处理", "URGENT": "尽快核实",
}

func reportLabel(code string) string {
	if label := reportLabels[code]; label != "" {
		return label
	}
	if code == "" {
		return "—"
	}
	return code
}
