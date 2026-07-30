<template>
  <div class="not-found">
    <div class="content">
      <h1>404</h1>
      <p>页面不存在</p>
      <el-button type="primary" @click="$router.push('/admin')">返回首页</el-button>
    </div>
  </div>
</template>

<style scoped>
.not-found {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 60vh;
}
.content {
  text-align: center;
}
h1 {
  font-size: 72px;
  color: #87CEEB;
  margin-bottom: 16px;
}
p {
  font-size: 18px;
  color: #909399;
  margin-bottom: 24px;
}
</style>
