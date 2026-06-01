interface CycleBadgeProps {
  position: number
  size: number
}

export function CycleBadge({ position, size }: CycleBadgeProps) {
  if (position === size) {
    return (
      <span className="inline-flex items-center justify-center w-4 h-4 rounded-full bg-red-500 text-white text-[9px] font-bold leading-none shrink-0">
        {position}
      </span>
    )
  }
  return (
    <span className="text-[9px] font-semibold opacity-60 leading-none shrink-0">
      {position}
    </span>
  )
}
