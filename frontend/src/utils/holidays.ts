// 中国法定节假日配置（2025-2026年）
// 数据来源：国务院办公厅关于节假日安排的通知

const holidays = new Set([
  // 2025年
  "2025-01-01",
  "2025-01-28",
  "2025-01-29",
  "2025-01-30",
  "2025-01-31",
  "2025-02-01",
  "2025-02-02",
  "2025-02-03",
  "2025-02-04",
  "2025-04-04",
  "2025-04-05",
  "2025-04-06",
  "2025-05-01",
  "2025-05-02",
  "2025-05-03",
  "2025-05-04",
  "2025-05-05",
  "2025-05-31",
  "2025-06-01",
  "2025-06-02",
  "2025-10-01",
  "2025-10-02",
  "2025-10-03",
  "2025-10-04",
  "2025-10-05",
  "2025-10-06",
  "2025-10-07",

  // 2026年
  "2026-01-01",
  "2026-01-02",
  "2026-01-03",
  "2026-02-17",
  "2026-02-18",
  "2026-02-19",
  "2026-02-20",
  "2026-02-21",
  "2026-02-22",
  "2026-02-23",
  "2026-04-04",
  "2026-04-05",
  "2026-04-06",
  "2026-05-01",
  "2026-05-02",
  "2026-05-03",
  "2026-05-04",
  "2026-05-05",
  "2026-06-22",
  "2026-06-23",
  "2026-06-24",
  "2026-09-27",
  "2026-10-01",
  "2026-10-02",
  "2026-10-03",
  "2026-10-04",
  "2026-10-05",
  "2026-10-06",
  "2026-10-07",
  "2026-10-08",
]);

/**
 * 判断是否为中国法定节假日
 * @param date 日期字符串（YYYY-MM-DD格式）或Date对象
 * @returns 是否为节假日
 */
export const isHoliday = (date: string | Date): boolean => {
  const dateStr = typeof date === "string" ? date : formatDate(date);
  return holidays.has(dateStr);
};

/**
 * 判断是否为A股交易日
 * A股交易规则：周一到周五交易，周六周日不交易（即使是调休补班日），法定节假日不交易
 * @param date Date对象
 * @returns 是否为交易日
 */
export const isTradingDay = (date: Date): boolean => {
  // 1. 周末不交易
  const day = date.getDay();
  if (day === 0 || day === 6) {
    return false;
  }

  // 2. 法定节假日不交易
  const dateStr = formatDate(date);
  if (holidays.has(dateStr)) {
    return false;
  }

  // 3. 周一到周五且不是节假日，是交易日
  return true;
};

/**
 * 格式化日期为 YYYY-MM-DD 格式
 */
const formatDate = (date: Date): string => {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
};

/**
 * 获取从指定日期开始的未来N个交易日
 * @param startDate 起始日期
 * @param count 需要的交易日数量
 * @returns 交易日数组
 */
export const getNextTradingDays = (
  startDate: Date,
  count: number
): Date[] => {
  const result: Date[] = [];
  const current = new Date(startDate);

  while (result.length < count) {
    if (isTradingDay(current)) {
      result.push(new Date(current));
    }
    current.setDate(current.getDate() + 1);
  }

  return result;
};
