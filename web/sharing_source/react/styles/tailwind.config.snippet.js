/**
 * Đoạn mã cấu hình mẫu cho tailwind.config.js của dự án bạn
 * Đảm bảo các font chữ Sans và Serif hoạt động đúng với thiết kế Luxury Editorial
 */

module.exports = {
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'Outfit', 'system-ui', '-apple-system', 'sans-serif'],
        serif: ['Cormorant Garamond', 'Playfair Display', 'Georgia', 'serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      colors: {
        neutral: {
          950: '#0a0a0a',
        }
      }
    },
  },
};
