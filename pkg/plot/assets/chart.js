function unpack(rows, key) {
    return rows.map(function (row) {
        return row[key];
    });
}

document.addEventListener("DOMContentLoaded", function () {
    const candleStickData = {
        name: "Candles",
        x: unpack(candles, "time"),
        close: unpack(candles, "close"),
        open: unpack(candles, "open"),
        low: unpack(candles, "low"),
        high: unpack(candles, "high"),
        type: "candlestick",
        xaxis: "x",
        yaxis: "y",
    };

    const points = [];
    const annotations = [];
    candles.forEach((candle) => {
        candle.orders.forEach(order => {
            const point = {
                time: candle.time,
                position: order.price,
                side: order.side,
                color: "green"
            }
            if (order.side === "SELL") {
                point.color = "red"
            }
            points.push(point);

            const annotation = {
                x: candle.time,
                y: candle.low,
                xref: "x",
                yref: "y",
                text: "B",
                hovertext: `${order.time}
                    <br>ID: ${order.id}
                    <br>Price: ${order.price.toLocaleString()}
                    <br>Size: ${order.quantity.toPrecision(4).toLocaleString()}
                    <br>Type: ${order.type}
                    <br>${(order.profit && "Profit: " + (order.profit * 100).toPrecision(2).toLocaleString() + "%") || ""}`,
                showarrow: true,
                arrowcolor: "green",
                valign: "bottom",
                borderpad: 4,
                arrowhead: 2,
                ax: 0,
                ay: 20,
                font: {
                    size: 12,
                    color: "green",
                },
            };

            if (order.side === "SELL") {
                annotation.font.color = "red";
                annotation.arrowcolor = "red";
                annotation.text = "S";
                annotation.y = candle.high;
                annotation.ay = -20;
                annotation.valign = "top";
            }
            annotations.push(annotation);
        });
    });

    const sellPoints = points.filter(p => p.side === "SELL");
    const buyPoints = points.filter(p => p.side === "BUY");
    const buyData = {
        name: "Buy Points",
        x: unpack(buyPoints, "time"),
        y: unpack(buyPoints, "position"),
        mode: 'markers',
        type: 'scatter',
        marker: {
            color: "green",
        }
    };
    const sellData = {
        name: "Sell Points",
        x: unpack(sellPoints, "time"),
        y: unpack(sellPoints, "position"),
        mode: 'markers',
        type: 'scatter',
        marker: {
            color: "red",
        }
    };

    var layout = {
        dragmode: "pan",
        margin: {
            r: 10,
            t: 25,
            b: 40,
            l: 60,
        },
        showlegend: true,
        xaxis: {
            autorange: true
        },
        yaxis: {
            autorange: true
        },
        annotations: annotations,
    };

    Plotly.newPlot("graph", [candleStickData, buyData, sellData], layout);
});
