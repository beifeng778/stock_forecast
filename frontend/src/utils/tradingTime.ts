import dayjs, { type Dayjs } from "dayjs";
import { isTradingDay as isTradingDayFromHolidays } from "./holidays";

export const getHHMM = (d: Date): number => d.getHours() * 100 + d.getMinutes();

export const isTradingDay = (d: Date = new Date()): boolean => {
  return isTradingDayFromHolidays(d);
};

export const isMarketClosed = (d: Date = new Date()): boolean => {
  return getHHMM(d) >= 1500;
};

export const isOpenToClose = (d: Date = new Date()): boolean => {
  if (!isTradingDay(d)) return false;
  const hhmm = getHHMM(d);
  return hhmm >= 930 && hhmm < 1500;
};

export const isTradingTime = (d: Date = new Date()): boolean => {
  if (!isTradingDay(d)) return false;
  const hhmm = getHHMM(d);
  const morning = hhmm >= 930 && hhmm < 1130;
  const afternoon = hhmm >= 1300 && hhmm < 1500;
  return morning || afternoon;
};

export const isWorkday = (date: Dayjs): boolean => {
  return isTradingDayFromHolidays(date.toDate());
};

export const isFutureDate = (date: Dayjs): boolean => {
  const today = dayjs().startOf("day");
  if (date.isAfter(today, "day")) return true;
  if (date.isSame(today, "day") && !isMarketClosed()) return true;
  return false;
};

export const getNextWorkdays = (count: number): Dayjs[] => {
  const workdays: Dayjs[] = [];
  let current = dayjs();
  if (isMarketClosed() || !isWorkday(current)) {
    current = current.add(1, "day");
  }
  while (workdays.length < count) {
    if (isWorkday(current)) {
      workdays.push(current);
    }
    current = current.add(1, "day");
  }
  return workdays;
};
