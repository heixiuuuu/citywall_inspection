/**
 * 登录态管理：token 的存储、读取与清理。
 * 正式接入微信登录后，token 来自 POST /api/v1/auth/wechat-login 的响应；
 * 业务原型阶段可使用测试身份。
 */

const TOKEN_KEY = 'city_wall_token'

export function setToken(token) {
  uni.setStorageSync(TOKEN_KEY, token)
}

export function getToken() {
  return uni.getStorageSync(TOKEN_KEY) || ''
}

export function clearToken() {
  uni.removeStorageSync(TOKEN_KEY)
}

export function isLoggedIn() {
  return Boolean(getToken())
}
