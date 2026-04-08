import React, { useState } from 'react';

interface Props {
  title: string;
}

/**
 * MyComponent handles some logic.
 */
export const MyComponent: React.FC<Props> = ({ title }) => {
  const [count, setCount] = useState(0);

  const increment = () => setCount(c => c + 1);

  return (
    <div onClick={increment}>
      <h1>{title}</h1>
      <p>Count: {count}</p>
    </div>
  );
};

export function useMyHook(initial: number) {
  const [val, setVal] = useState(initial);
  return [val, setVal] as const;
}

class OldComponent extends React.Component {
    render() {
        return <div>Old</div>;
    }
}
