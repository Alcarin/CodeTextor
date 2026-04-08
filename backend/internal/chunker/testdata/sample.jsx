import React, { useState } from 'react';

/**
 * Un componente funzionale che utilizza hook e tag JSX annidati
 */
const Counter = ({ initialValue = 0 }) => {
  const [count, setCount] = useState(initialValue);

  const handleIncrement = () => {
    setCount(prev => prev + 1);
  };

  return (
    <div className="counter-container">
      <h3>Conteggio attuale: {count}</h3>
      <div className="button-group">
        <button onClick={handleIncrement}>Incrementa</button>
        <button onClick={() => setCount(0)}>Reset</button>
      </div>
      {count > 10 && <span className="warning">Attenzione: conteggio elevato!</span>}
    </div>
  );
};

export default Counter;

class LegacyWelcome extends React.Component {
  render() {
    return (
      <section>
        <h1>Benvenuti nel componente Class-based</h1>
        <Counter initialValue={10} />
      </section>
    );
  }
}
