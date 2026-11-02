import assert from 'node:assert/strict'
import { after, before, beforeEach, test } from 'node:test'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'

let server
let uploadImage
let uploadOptions
let storage
let toasts
let navigations

before(async () => {
  server = await createServer({
    root: fileURLToPath(new URL('..', import.meta.url)),
    configFile: false,
    server: { middlewareMode: true },
  })
  ;({ uploadImage } = await server.ssrLoadModule('/src/shared/api/request.js'))
})

after(async () => {
  await server?.close()
  delete globalThis.uni
})

beforeEach(() => {
  storage = new Map([
    ['city_wall_token', 'test-token'],
    ['city_wall_user', { id: 3 }],
    ['city_wall_api_base', 'http://localhost:18080/api/v1'],
  ])
  toasts = []
  navigations = []
  globalThis.uni = {
    getStorageSync: key => storage.get(key),
    removeStorageSync: key => storage.delete(key),
    showToast: options => toasts.push(options),
    reLaunch: options => navigations.push(options),
    uploadFile: options => { uploadOptions = options },
  }
})

test('上传携带凭据、角色与超时，返回图片数据', async () => {
  const pending = uploadImage(12, '/photo.jpg', 'OVERVIEW')
  assert.equal(uploadOptions.url, 'http://localhost:18080/api/v1/observations/12/images')
  assert.equal(uploadOptions.header.Authorization, 'Bearer test-token')
  assert.ok(uploadOptions.header['X-Request-Id'])
  assert.equal(uploadOptions.timeout, 110000)
  assert.equal(uploadOptions.filePath, '/photo.jpg')
  assert.equal(uploadOptions.name, 'image')
  assert.deepEqual(uploadOptions.formData, { image_role: 'OVERVIEW' })
  const body = { data: { image: { id: 9 } } }
  uploadOptions.success({ statusCode: 201, data: JSON.stringify(body) })
  assert.deepEqual(await pending, body)
})

for (const statusCode of [200, 502]) {
  test(`${statusCode} 非 JSON 响应结束请求、释放调用方状态并可重试`, { timeout: 2000 }, async () => {
    let busy = true
    const pending = uploadImage(12, '/photo.jpg', 'OVERVIEW').finally(() => { busy = false })
    assert.doesNotThrow(() => uploadOptions.success({ statusCode, data: '<html>Bad Gateway</html>' }))
    await assert.rejects(pending, error => {
      assert.equal(error.statusCode, statusCode)
      assert.equal(error.data.error.code, 'INVALID_RESPONSE')
      return true
    })
    assert.equal(busy, false)
    busy = true
    const retry = uploadImage(12, '/photo.jpg', 'OVERVIEW').finally(() => { busy = false })
    const body = { data: { image: { id: 10 } } }
    uploadOptions.success({ statusCode: 201, data: JSON.stringify(body) })
    assert.deepEqual(await retry, body)
    assert.equal(busy, false)
  })
}

for (const data of ['{"error":{"code":"UNAUTHORIZED"}}', '<html>Unauthorized</html>']) {
  test(`401 清除登录态并跳登录页：${data.startsWith('<') ? '非 JSON' : 'JSON'}`, async () => {
    const pending = uploadImage(12, '/photo.jpg', 'DETAIL')
    assert.doesNotThrow(() => uploadOptions.success({ statusCode: 401, data }))
    await assert.rejects(pending, error => error.statusCode === 401)
    assert.equal(storage.has('city_wall_token'), false)
    assert.equal(storage.has('city_wall_user'), false)
    assert.deepEqual(navigations, [{ url: '/pages/index/index' }])
    assert.equal(toasts[0].title, '登录已过期，请重新登录')
  })
}

test('JSON 业务错误保留状态码与服务端信息', async () => {
  const pending = uploadImage(12, '/photo.jpg', 'DETAIL')
  const body = { error: { code: 'IMAGE_LIMIT_EXCEEDED', message: '图片数量超限' } }
  uploadOptions.success({ statusCode: 422, data: JSON.stringify(body) })
  await assert.rejects(pending, error => {
    assert.deepEqual(error, { statusCode: 422, data: body })
    return true
  })
})

test('网络失败或超时结束请求并释放调用方状态', async () => {
  let busy = true
  const pending = uploadImage(12, '/photo.jpg', 'DETAIL').finally(() => { busy = false })
  const failure = { errMsg: 'uploadFile:fail timeout' }
  uploadOptions.fail(failure)
  await assert.rejects(pending, error => error === failure)
  assert.equal(busy, false)
})
