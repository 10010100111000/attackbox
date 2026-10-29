// 共享的 Select 交互助手:reka-ui 的触发器不是原生 <select>,
// 浏览器测试改为「点开触发器 + 点选项」。
export async function chooseOption(page, trigger, optionName) {
  await trigger.click()
  await page.getByRole('option', { name: optionName, exact: true }).click()
}
