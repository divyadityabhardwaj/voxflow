import React, { useState, useRef, useEffect } from "react";

interface TooltipProps {
  children: React.ReactNode;
  content: string;
  position?: "top" | "bottom" | "left" | "right";
  delay?: number;
}

export const Tooltip: React.FC<TooltipProps> = ({
  children,
  content,
  position = "bottom",
  delay = 300,
}) => {
  const [isVisible, setIsVisible] = useState(false);
  const [coords, setCoords] = useState({ x: 0, y: 0 });
  const triggerRef = useRef<HTMLDivElement>(null);
  const timeoutRef = useRef<ReturnType<typeof setTimeout>>();

  const handleMouseEnter = () => {
    timeoutRef.current = setTimeout(() => {
      if (triggerRef.current) {
        const rect = triggerRef.current.getBoundingClientRect();
        let x = 0,
          y = 0;

        switch (position) {
          case "bottom":
            x = rect.left + rect.width / 2;
            y = rect.bottom + 8;
            break;
          case "top":
            x = rect.left + rect.width / 2;
            y = rect.top - 8;
            break;
          case "left":
            x = rect.left - 8;
            y = rect.top + rect.height / 2;
            break;
          case "right":
            x = rect.right + 8;
            y = rect.top + rect.height / 2;
            break;
        }

        setCoords({ x, y });
        setIsVisible(true);
      }
    }, delay);
  };

  const handleMouseLeave = () => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
    setIsVisible(false);
  };

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return (
    <>
      <div
        ref={triggerRef}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
        className="inline-flex"
      >
        {children}
      </div>
      {isVisible && (
        <div
          className="fixed z-[100] px-3 py-1.5 bg-bg-tertiary text-primary text-xs rounded-lg shadow-lg border border-border pointer-events-none whitespace-nowrap"
          style={{
            left: coords.x,
            top: coords.y,
            transform:
              position === "bottom" || position === "top"
                ? "translateX(-50%)"
                : position === "left"
                  ? "translate(-100%, -50%)"
                  : "translateY(-50%)",
          }}
        >
          {content}
        </div>
      )}
    </>
  );
};
