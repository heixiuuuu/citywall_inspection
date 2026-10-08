/**
 * HTTP 请求封装（uni.request）。
 * 约定（与 server/api/openapi.yaml 对齐）：
 *  - 统一附带 Authorization: Bearer <token> 与 X-Request-Id；
 *  - 成功（2xx）resolve res.data；失败 reject {statusCode, error:{code,message}, request_id}；
 *  - 401 清空登录态并跳登录页；409 提示冲突（revision 过期）后回退重取；
 *  - 服务端返回成功前不得显示"已提交"。
 */
import { getToken, clearToken, getUser } from '../auth/auth'

// dev 联调地址；正式发布按环境注入（HBuilderX 环境变量 / 打包时替换）。
// - 微信开发者工具 / HBuilderX 内置模拟器：127.0.0.1 可直接访问电脑上的 Go 服务。
// - Android 真机（APK）：127.0.0.1 是手机自身，必须改为电脑的局域网 IP，
//   例如 http://192.168.1.100:8080/api/v1，并把值写到本地存储 key `city_wall_api_base`（见下方）。
export const BASE_URL = 'http://127.0.0.1:8080/api/v1'

// App（APK）真机联调：允许通过本地存储覆盖服务地址，避免改代码重新打包。
// 首次装机时可在任意页面执行：
//   uni.setStorageSync('city_wall_api_base', 'http://<电脑局域网IP>:8080/api/v1')
export function getBaseUrl() {
  // #ifdef APP-PLUS
  const saved = uni.getStorageSync('city_wall_api_base')
  if (saved) return saved
  // #endif
  // #ifdef H5
  const savedH5 = uni.getStorageSync('city_wall_api_base')
  if (savedH5) return savedH5
  // H5 在 Android 模拟器（WebView 壳）中访问宿主机 Go 服务
  return 'http://10.0.2.2:8080/api/v1'
  // #endif
  return BASE_URL
}

function notify(title) {
  uni.showToast({ title, icon: 'none', duration: 2200 })
}

function toLogin() {
  clearToken()
  uni.reLaunch({ url: '/pages/index/index' })
}

export function request(path, { method = 'GET', data } = {}) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    uni.request({
      url: `${getBaseUrl()}${path}`,
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
          return
        }
        const code = res.data && res.data.error && res.data.error.code
        if (res.statusCode === 401) {
          notify('登录已过期，请重新登录')
          toLogin()
          reject(res.data)
          return
        }
        if (res.statusCode === 409) {
          notify(code || '数据已更新，请重试')
        } else {
          notify((res.data && res.data.error && res.data.error.message) || '请求失败')
        }
        reject({ statusCode: res.statusCode, data: res.data })
      },
      fail(err) {
        notify('网络异常，请检查服务是否启动')
        reject(err)
      },
    })
  })
}

/** 上传图片：multipart/form-data，返回 {data:{image}}。 */
export function uploadImage(observationId, localPath, imageRole) {
  return new Promise((resolve, reject) => {
    const token = getToken()
    uni.uploadFile({
      url: `${getBaseUrl()}/observations/${observationId}/images`,
      filePath: localPath,
      name: 'image',
      formData: { image_role: imageRole },
      header: { ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      success(res) {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(JSON.parse(res.data))
          return
        }
        reject({ statusCode: res.statusCode, data: JSON.parse(res.data) })
      },
      fail: reject,
    })
  })
}

/** 带 token 获取受控图片内容（返回临时路径）。 */
export function loadImageContent(imageId) {
  const token = getToken()
  if (!token) return Promise.reject(new Error('no token'))
  return new Promise((resolve, reject) => {
    uni.downloadFile({
      url: `${getBaseUrl()}/images/${imageId}/content`,
      header: { Authorization: `Bearer ${token}` },
      success(res) {
        if (res.statusCode === 200) resolve(res.tempFilePath)
        else reject(new Error(`load image ${res.statusCode}`))
      },
      fail: reject,
    })
  })
}

/** 当前登录用户（本地缓存）。 */
export function currentUser() {
  return getUser()
}

function genRequestId() {
  return `req_${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`
}
