import { request, uploadImage } from './request'

/** 现场记录相关接口（与 server/api/openapi.yaml 契约对齐）。 */

/**
 * POST /observations 创建现场记录草稿。
 * source: TASK（必带 task_point_id）/ VOLUNTARY（不得带）。
 */
export function createObservation(source, taskPointId, locationText) {
  return request('/observations', {
    method: 'POST',
    data:
      source === 'TASK'
        ? { source, task_point_id: taskPointId, location_text: locationText }
        : { source, location_text: locationText },
  })
}

/** POST /observations/{id}/images 上传图片（概览/细节）。 */
export function addImage(observationId, localPath, imageRole) {
  return uploadImage(observationId, localPath, imageRole)
}

/** PATCH /observations/{id} 更新草稿位置/现场判断。 */
export function updateObservation(id, revision, patch) {
  return request(`/observations/${id}`, { method: 'PATCH', data: { revision, ...patch } })
}

/** POST /images/{id}/recognitions 触发识别尝试。 */
export function recognize(imageId) {
  return request(`/images/${imageId}/recognitions`, { method: 'POST', data: {} })
}

/** PUT /observations/{id}/conclusion 保存人工结论。 */
export function saveConclusion(observationId, revision, conclusion) {
  return request(`/observations/${observationId}/conclusion`, {
    method: 'PUT',
    data: { revision, ...conclusion },
  })
}

/** POST /observations/{id}/submit 提交记录。 */
export function submitObservation(observationId, revision) {
  return request(`/observations/${observationId}/submit`, { method: 'POST', data: { revision } })
}

/** GET /inspection-points 点位列表（巡检端只读）。 */
export function getInspectionPoints() {
  return request('/inspection-points')
}
