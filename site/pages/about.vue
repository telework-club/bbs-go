<template>
  <section class="main">
    <div class="container">
      <div class="widget">
        <div class="widget-header">关于</div>
        <div class="widget-content content">
          <h2>简介</h2>
          <p>BBS-GO&nbsp;是一款基于Go语言研发的精美小巧的开源社区论坛系统。</p>

          <p>
            Telework在BBS-Go的基础上，对功能进行了一部分修改，形成了现在的社区系统。
          </p>

          <h2>与我联系</h2>
          <p>
            如有需要可以在
            <a href="https://github.com/telework-club/forum/issues">Github</a>
            上发issue，也可以直接发邮件
            <a href="mailto://core.telework.club?subject=反馈">联系我们</a>
          </p>
          <p>
            联系原项目作者可前往&nbsp;<a
              href="https://mlog.club"
              target="_blank"
              >码农俱乐部</a
            >&nbsp;交流。
          </p>
          <h2>原项目贡献者</h2>
          <p>
            <a
              href="https://github.com/mlogclub/bbs-go/graphs/contributors"
              target="_blank"
              >Contributors</a
            >
          </p>
          <!-- 展示广告 -->
          <adsbygoogle ad-slot="1742173616" />
        </div>
      </div>
    </div>
  </section>
</template>

<script>
export default {
  head() {
    return {
      title: this.$siteTitle('关于'),
    }
  },
}
</script>

<style lang="scss" scoped></style>
