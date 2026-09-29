/**
 * HTTP 请求封装（uni.request）。
 * 约定：
 *  - 统一附带 Authorization: Bearer <token> 与 X-Request-Id；
 *  - 401/403/409 与 5xx 做不同反馈，409 后重新拉取记录供用户判断；
 *  - 服务端返回成功前不得显示“已提交”。
 */
import { getToken } from '../auth/auth'

const BASE_URL = 'http://127.0.0.1:8080/api/v1'

export function request(path, { method = 'GET', data } = {}) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    uni.request({
      url: `${BASE_URL}${path}`,
      method,
      data,
      header: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        'X-Request-Id': genRequestId(),
      },
      success(res) {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data)
        } else {
          // TODO: 按 error.code 分类提示（VALIDATION_ERROR / CONFLICT / …）
          reject(res.data)
        }
      },
      fail(err) {
        reject(err)
      },
    })
  })
}

/** 上传图片：multipart/form-data，返回 image_id。 */
export function uploadImage(observationId, localPath, imageRole) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    uni.uploadFile({
      url: `${BASE_URL}/observations/${observationId}/images`,
      filePath: localPath,
      name: 'image',
      formData: { image_role: imageRole, idempotency_key: genRequestId() },
      header: { ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      success(res) {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(JSON.parse(res.data))
        } else {
          reject(JSON.parse(res.data))
        }
      },
      fail: reject,
    })
  })
}

function genRequestId() {
  return `req_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`
}
