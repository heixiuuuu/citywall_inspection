/**
 * 登录态管理：token 与用户信息的存储、读取与清理。
 * 正式接入微信登录后，token 来自 POST /api/v1/auth/wechat-login；
 * MVP 阶段小程序端用测试 code（test-inspector 等）换取身份。
 */

const TOKEN_KEY = 'city_wall_token'
const USER_KEY = 'city_wall_user'

export function setToken(token) {
  uni.setStorageSync(TOKEN_KEY, token)
}

export function getToken() {
  return uni.getStorageSync(TOKEN_KEY) || ''
}

export function clearToken() {
  uni.removeStorageSync(TOKEN_KEY)
  uni.removeStorageSync(USER_KEY)
}

export function setUser(user) {
  uni.setStorageSync(USER_KEY, user)
}

export function getUser() {
  return uni.getStorageSync(USER_KEY) || null
}

export function isLoggedIn() {
  return Boolean(getToken())
}
