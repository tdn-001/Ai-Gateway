import { ref, onMounted, onUnmounted } from 'vue'

export function useResponsive() {
  const isMobile = ref(window.innerWidth < 768)
  const isTablet = ref(window.innerWidth < 992)

  const onResize = () => {
    isMobile.value = window.innerWidth < 768
    isTablet.value = window.innerWidth < 992
  }

  onMounted(() => {
    window.addEventListener('resize', onResize)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', onResize)
  })

  return { isMobile, isTablet }
}
