interface CycleBadgeProps {
  position: number
  size: number
}

export function CycleBadge({ position, size }: CycleBadgeProps) {
  if (position === size) {
    return (
      <span className="inline-flex items-center justify-center w-4 h-4 rounded-full bg-red-700 text-white text-[10px] font-bold leading-none shrink-0">
        {position}
      </span>
    )
  }
  return (
    <span className="text-[10px] font-semibold opacity-60 leading-none shrink-0">
      {position}
    </span>
  )
}