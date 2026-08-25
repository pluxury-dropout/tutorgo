"use client"

import * as React from "react"
import { Popover as PopoverPrimitive } from "@base-ui/react/popover"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { XIcon } from "lucide-react"

/** Якорь: живой DOM-узел либо готовый прямоугольник (для эфемерных целей вроде
 *  выделенного слота календаря, который исчезает сразу после клика). */
type PopoverAnchor = Element | DOMRect

function Popover({ ...props }: PopoverPrimitive.Root.Props) {
  return <PopoverPrimitive.Root data-slot="popover" {...props} />
}

function PopoverContent({
  anchor,
  side = "left",
  align = "start",
  sideOffset = 8,
  collisionPadding = 12,
  className,
  children,
  showCloseButton = true,
  ...props
}: PopoverPrimitive.Popup.Props &
  Pick<
    PopoverPrimitive.Positioner.Props,
    "side" | "align" | "sideOffset" | "collisionPadding"
  > & {
    anchor?: PopoverAnchor | null
    showCloseButton?: boolean
  }) {
  // Узел, к которому привязались, может пересоздаться, пока поповер открыт
  // (FullCalendar перерисовывает события на каждый ререндер). Тогда держим
  // последний известный прямоугольник — иначе поповер отвяжется и улетит в угол.
  const anchorEl = React.useMemo(() => {
    if (!anchor) return null
    if (!(anchor instanceof Element)) return { getBoundingClientRect: () => anchor }
    const last = anchor.getBoundingClientRect()
    return {
      contextElement: anchor,
      getBoundingClientRect: () =>
        anchor.isConnected ? anchor.getBoundingClientRect() : last,
    }
  }, [anchor])

  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Positioner
        className="isolate z-50 outline-none"
        anchor={anchorEl}
        side={side}
        align={align}
        sideOffset={sideOffset}
        collisionPadding={collisionPadding}
      >
        <PopoverPrimitive.Popup
          data-slot="popover-content"
          className={cn(
            "relative grid max-h-(--available-height) w-[min(22rem,calc(100vw-2rem))] origin-(--transform-origin) gap-4 overflow-x-hidden overflow-y-auto rounded-xl bg-popover p-4 text-sm text-popover-foreground shadow-lg ring-1 ring-foreground/10 duration-100 outline-none data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2 data-open:animate-in data-open:fade-in-0 data-open:zoom-in-95 data-closed:animate-out data-closed:fade-out-0 data-closed:zoom-out-95",
            className
          )}
          {...props}
        >
          {children}
          {showCloseButton && (
            <PopoverPrimitive.Close
              data-slot="popover-close"
              render={
                <Button
                  variant="ghost"
                  className="absolute top-2 right-2"
                  size="icon-sm"
                />
              }
            >
              <XIcon />
              <span className="sr-only">Close</span>
            </PopoverPrimitive.Close>
          )}
        </PopoverPrimitive.Popup>
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  )
}

function PopoverTitle({ className, ...props }: PopoverPrimitive.Title.Props) {
  return (
    <PopoverPrimitive.Title
      data-slot="popover-title"
      className={cn(
        "font-heading text-base leading-none font-medium",
        className
      )}
      {...props}
    />
  )
}

export { Popover, PopoverContent, PopoverTitle }
